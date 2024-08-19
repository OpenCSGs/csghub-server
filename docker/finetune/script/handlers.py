"""
Authenticated HTTP proxy for Jupyter Notebooks

Some original inspiration from https://github.com/senko/tornado-proxy
"""

import os
import json
import re
import socket
from asyncio import Lock
from copy import copy
from tempfile import mkdtemp
from urllib.parse import quote, urlparse, urlunparse

import aiohttp
from jupyter_server.base.handlers import JupyterHandler, utcnow
from jupyter_server.utils import ensure_async, url_path_join
from simpervisor import SupervisedProcess
from tornado import httpclient, httputil, web
from tornado.simple_httpclient import SimpleAsyncHTTPClient
from traitlets import Bytes, Dict, Instance, Integer, Unicode, Union, default, observe
from traitlets.traitlets import HasTraits

from .unixsock import UnixResolver
from .utils import call_with_asked_args
from .websocket import WebSocketHandlerMixin, pingable_ws_connect


class RewritableResponse(HasTraits):
    """
    A class to hold the response to be rewritten by rewrite_response
    """

    # The following should not be modified (or even accessed) by rewrite_response.
    # It is used to initialize the default values of the traits.
    orig_response = Instance(klass=httpclient.HTTPResponse)

    # The following are modifiable by rewrite_response
    headers = Union(trait_types=[Dict(), Instance(klass=httputil.HTTPHeaders)])
    body = Bytes()
    code = Integer()
    reason = Unicode(allow_none=True)

    @default("headers")
    def _default_headers(self):
        return copy(self.orig_response.headers)

    @default("body")
    def _default_body(self):
        return self.orig_response.body

    @default("code")
    def _default_code(self):
        return self.orig_response.code

    @default("reason")
    def _default_reason(self):
        return self.orig_response.reason

    @observe("code")
    def _observe_code(self, change):
        # HTTP status codes are mapped to short descriptions in the
        # httputil.responses dictionary, 200 maps to "OK", 403 maps to
        # "Forbidden" etc.
        #
        # If code is updated and it previously had a reason matching its short
        # description, we update reason to match the new code's short
        # description.
        #
        if self.reason == httputil.responses.get(change["old"], "Unknown"):
            self.reason = httputil.responses.get(change["new"], "Unknown")

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        # Trigger the default value to be set from orig_response on instantiation.
        # Otherwise _observe_code will receive change['old'] == 0.
        self.code

    def _apply_to_copy(self, func):
        """
        Apply a function to a copy of self, and return the copy
        """
        new = copy(self)
        func(new)
        return new


class AddSlashHandler(JupyterHandler):
    """Add trailing slash to URLs that need them."""

    @web.authenticated
    def get(self, *args):
        src = urlparse(self.request.uri)
        dest = src._replace(path=src.path + "/")
        self.redirect(urlunparse(dest))


class ProxyHandler(WebSocketHandlerMixin, JupyterHandler):
    """
    A tornado request handler that proxies HTTP and websockets from
    a given host/port combination. This class is not meant to be
    used directly as a means of overriding CORS. This presents significant
    security risks, and could allow arbitrary remote code access. Instead, it is
    meant to be subclassed and used for proxying URLs from trusted sources.

    Subclasses should implement open, http_get, post, put, delete, head, patch,
    and options.
    """

    unix_socket = None  # Used in subclasses

    def __init__(self, *args, **kwargs):
        self.proxy_base = ""
        self.absolute_url = kwargs.pop("absolute_url", False)
        self.host_allowlist = kwargs.pop("host_allowlist", ["localhost", "127.0.0.1"])
        self.rewrite_response = kwargs.pop(
            "rewrite_response",
            tuple(),
        )
        self._requested_subprotocols = None
        super().__init__(*args, **kwargs)

    # Support/use jupyter_server config arguments allow_origin and allow_origin_pat
    # to enable cross origin requests propagated by e.g. inverting proxies.

    def check_origin(self, origin=None):
        return JupyterHandler.check_origin(self, origin)

    # Support all the methods that tornado does by default except for GET which
    # is passed to WebSocketHandlerMixin and then to WebSocketHandler.

    async def open(self, port, proxied_path):
        raise NotImplementedError("Subclasses of ProxyHandler should implement open")

    async def prepare(self, *args, **kwargs):
        """
        Enforce authentication on *all* requests.

        This method is called *before* any other method for all requests.
        See https://www.tornadoweb.org/en/stable/web.html#tornado.web.RequestHandler.prepare.
        """
        # Due to https://github.com/jupyter-server/jupyter_server/issues/1012,
        # we can not decorate `prepare` with `@web.authenticated`.
        # `super().prepare`, which calls `JupyterHandler.prepare`, *must* be called
        # before `@web.authenticated` can work. Since `@web.authenticated` is a decorator
        # that relies on the decorated method to get access to request information, we can
        # not call it directly. Instead, we create an empty lambda that takes a request_handler,
        # decorate that with web.authenticated, and call the decorated function.
        # super().prepare became async with jupyter_server v2
        _prepared = super().prepare(*args, **kwargs)
        if _prepared is not None:
            await _prepared

        # If this is a GET request that wants to be upgraded to a websocket, users not
        # already authenticated gets a straightforward 403. Everything else is dealt
        # with by `web.authenticated`, which does a 302 to the appropriate login url.
        # Websockets are purely API calls made by JS rather than a direct user facing page,
        # so redirects do not make sense for them.
        if (
            self.request.method == "GET"
            and self.request.headers.get("Upgrade", "").lower() == "websocket"
        ):
            if not self.current_user:
                raise web.HTTPError(403)
        else:
            web.authenticated(lambda request_handler: None)(self)

    async def http_get(self, host, port, proxy_path=""):
        """Our non-websocket GET."""
        raise NotImplementedError(
            "Subclasses of ProxyHandler should implement http_get"
        )

    def post(self, host, port, proxy_path=""):
        raise NotImplementedError(
            "Subclasses of ProxyHandler should implement this post"
        )

    def put(self, port, proxy_path=""):
        raise NotImplementedError(
            "Subclasses of ProxyHandler should implement this put"
        )

    def delete(self, host, port, proxy_path=""):
        raise NotImplementedError("Subclasses of ProxyHandler should implement delete")

    def head(self, host, port, proxy_path=""):
        raise NotImplementedError("Subclasses of ProxyHandler should implement head")

    def patch(self, host, port, proxy_path=""):
        raise NotImplementedError("Subclasses of ProxyHandler should implement patch")

    def options(self, host, port, proxy_path=""):
        raise NotImplementedError("Subclasses of ProxyHandler should implement options")

    def on_message(self, message):
        """
        Called when we receive a message from our client.

        We proxy it to the backend.
        """
        self._record_activity()
        if hasattr(self, "ws"):
            self.ws.write_message(message, binary=isinstance(message, bytes))

    def on_ping(self, data):
        """
        Called when the client pings our websocket connection.

        We proxy it to the backend.
        """
        self.log.debug(f"jupyter_server_proxy: on_ping: {data}")
        self._record_activity()
        if hasattr(self, "ws"):
            self.ws.protocol.write_ping(data)

    def on_pong(self, data):
        """
        Called when we receive a ping back.
        """
        self.log.debug(f"jupyter_server_proxy: on_pong: {data}")

    def on_close(self):
        """
        Called when the client closes our websocket connection.

        We close our connection to the backend too.
        """
        if hasattr(self, "ws"):
            self.ws.close()

    def _record_activity(self):
        """Record proxied activity as API activity

        avoids proxied traffic being ignored by the notebook's
        internal idle-shutdown mechanism
        """
        self.settings["api_last_activity"] = utcnow()

    def _get_context_path(self, host, port):
        """
        Some applications need to know where they are being proxied from.
        This is either:
        - {base_url}/proxy/{port}
        - {base_url}/proxy/{host}:{port}
        - {base_url}/proxy/absolute/{port}
        - {base_url}/proxy/absolute/{host}:{port}
        - {base_url}/{proxy_base}
        """
        host_and_port = str(port) if host == "localhost" else host + ":" + str(port)
        if self.proxy_base:
            return url_path_join(self.base_url, self.proxy_base)
        if self.absolute_url:
            return url_path_join(self.base_url, "proxy", "absolute", host_and_port)
        else:
            return url_path_join(self.base_url, "proxy", host_and_port)

    def get_client_uri(self, protocol, host, port, proxied_path):
        if self.absolute_url:
            context_path = self._get_context_path(host, port)
            client_path = url_path_join(context_path, proxied_path)
        else:
            client_path = proxied_path

        # ensure client_path always starts with '/'
        if not client_path.startswith("/"):
            client_path = "/" + client_path

        # Quote spaces, åäö and such, but only enough to send a valid web
        # request onwards. To do this, we mark the RFC 3986 specs' "reserved"
        # and "un-reserved" characters as safe that won't need quoting. The
        # un-reserved need to be marked safe to ensure the quote function behave
        # the same in py36 as py37.
        #
        # ref: https://tools.ietf.org/html/rfc3986#section-2.2
        client_path = quote(client_path, safe=":/?#[]@!$&'()*+,;=-._~")

        client_uri = "{protocol}://{host}:{port}{path}".format(
            protocol=protocol,
            host=host,
            port=port,
            path=client_path,
        )
        if self.request.query:
            client_uri += "?" + self.request.query

        return client_uri

    def _build_proxy_request(self, host, port, proxied_path, body, **extra_opts):
        headers = self.proxy_request_headers()

        client_uri = self.get_client_uri("http", host, port, proxied_path)
        # Some applications check X-Forwarded-Context and X-ProxyContextPath
        # headers to see if and where they are being proxied from.
        if not self.absolute_url:
            context_path = self._get_context_path(host, port)
            headers["X-Forwarded-Context"] = context_path
            headers["X-ProxyContextPath"] = context_path
            # to be compatible with flask/werkzeug wsgi applications
            headers["X-Forwarded-Prefix"] = context_path

        req = httpclient.HTTPRequest(
            client_uri,
            method=self.request.method,
            body=body,
            decompress_response=False,
            headers=headers,
            **self.proxy_request_options(),
            **extra_opts,
        )
        return req

    def _check_host_allowlist(self, host):
        if callable(self.host_allowlist):
            return self.host_allowlist(self, host)
        else:
            return host in self.host_allowlist

    async def proxy(self, host, port, proxied_path):
        """
        This serverextension handles:
            {base_url}/proxy/{port([0-9]+)}/{proxied_path}
            {base_url}/proxy/absolute/{port([0-9]+)}/{proxied_path}
            {base_url}/{proxy_base}/{proxied_path}
        """

        if not self._check_host_allowlist(host):
            raise web.HTTPError(
                403,
                f"Host '{host}' is not allowed. "
                "See https://jupyter-server-proxy.readthedocs.io/en/latest/arbitrary-ports-hosts.html for info.",
            )

        # Remove hop-by-hop headers that don't necessarily apply to the request we are making
        # to the backend. See https://github.com/jupyterhub/jupyter-server-proxy/pull/328
        # for more information
        hop_by_hop_headers = [
            "Proxy-Connection",
            "Keep-Alive",
            "Transfer-Encoding",
            "TE",
            "Connection",
            "Trailer",
            "Upgrade",
            "Proxy-Authorization",
            "Proxy-Authenticate",
        ]
        for header_to_remove in hop_by_hop_headers:
            if header_to_remove in self.request.headers:
                del self.request.headers[header_to_remove]

        self._record_activity()

        if self.request.headers.get("Upgrade", "").lower() == "websocket":
            # We wanna websocket!
            # jupyterhub/jupyter-server-proxy@36b3214
            self.log.info(
                "we wanna websocket, but we don't define WebSocketProxyHandler"
            )
            self.set_status(500)

        body = self.request.body
        if not body:
            if self.request.method in {"POST", "PUT"}:
                body = b""
            else:
                body = None
        accept_type = self.request.headers.get('Accept')
        if accept_type == 'text/event-stream':
            return await self._proxy_progressive(host, port, proxied_path, body)
        else:
            return await self._proxy_normal(host, port, proxied_path, body)

    async def _proxy_progressive(self, host, port, proxied_path, body):
        # Proxy in progressive flush mode, whenever chunks are received. Potentially slower but get results quicker for voila

        client = httpclient.AsyncHTTPClient(force_instance=True)

        # Set up handlers so we can progressively flush result

        headers_raw = []

        def dump_headers(headers_raw):
            for line in headers_raw:
                r = re.match('^([a-zA-Z0-9\-_]+)\s*\:\s*([^\r\n]+)[\r\n]*$', line)
                if r:
                    k, v = r.groups([1, 2])
                    if k not in ('Content-Length', 'Transfer-Encoding',
                                 'Content-Encoding', 'Connection'):
                        # some header appear multiple times, eg 'Set-Cookie'
                        self.set_header(k, v)
                else:
                    r = re.match('^HTTP[^\s]* ([0-9]+)', line)
                    if r:
                        status_code = r.group(1)
                        self.set_status(int(status_code))
            headers_raw.clear()

        # clear tornado default header
        self._headers = httputil.HTTPHeaders()

        def header_callback(line):
            headers_raw.append(line)

        def streaming_callback(chunk):
            # Do this here, not in header_callback so we can be sure headers are out of the way first
            dump_headers(headers_raw)  # array will be empty if this was already called before
            self.write(chunk)
            self.flush()

        # Now make the request

        req = self._build_proxy_request(host, port, proxied_path, body,
                                        streaming_callback=streaming_callback,
                                        header_callback=header_callback)

        # no timeout for stream api
        req.request_timeout = 7200
        req.connect_timeout = 600

        try:
            response = await client.fetch(req, raise_error=False)
        except httpclient.HTTPError as err:
            if err.code == 599:
                self._record_activity()
                self.set_status(599)
                self.write(str(err))
                return
            else:
                raise

        # record activity at start and end of requests
        self._record_activity()

        # For all non http errors...
        if response.error and type(response.error) is not httpclient.HTTPError:
            self.set_status(500)
            self.write(str(response.error))
        else:
            self.set_status(response.code, response.reason)  # Should already have been set

            dump_headers(headers_raw)  # Should already have been emptied

            if response.body:  # Likewise, should already be chunked out and flushed
                self.write(response.body)

    async def _proxy_normal(self, host, port, proxied_path, body):
        if self.unix_socket is not None:
            # Port points to a Unix domain socket
            self.log.debug("Making client for Unix socket %r", self.unix_socket)
            assert host == "localhost", "Unix sockets only possible on localhost"
            client = SimpleAsyncHTTPClient(
                force_instance=True, resolver=UnixResolver(self.unix_socket)
            )
        else:
            client = httpclient.AsyncHTTPClient(force_instance=True)

        req = self._build_proxy_request(host, port, proxied_path, body)

        self.log.debug(f"Proxying request to {req.url}")

        try:
            # Here, "response" is a tornado.httpclient.HTTPResponse object.
            response = await client.fetch(req, raise_error=False)
        except httpclient.HTTPError as err:
            # We need to capture the timeout error even with raise_error=False,
            # because it only affects the HTTPError raised when a non-200 response
            # code is used, instead of suppressing all errors.
            # Ref: https://www.tornadoweb.org/en/stable/httpclient.html#tornado.httpclient.AsyncHTTPClient.fetch
            if err.code == 599:
                self._record_activity()
                raise web.HTTPError(599, str(err))
            else:
                raise

        # record activity at start and end of requests
        self._record_activity()

        # For all non http errors...
        if response.error and type(response.error) is not httpclient.HTTPError:
            raise web.HTTPError(500, str(response.error))
        else:
            # Represent the original response as a RewritableResponse object.
            original_response = RewritableResponse(orig_response=response)

            # The function (or list of functions) which should be applied to modify the
            # response.
            rewrite_response = self.rewrite_response

            # If this is a single function, wrap it in a list.
            if isinstance(rewrite_response, (list, tuple)):
                rewrite_responses = rewrite_response
            else:
                rewrite_responses = [rewrite_response]

            # To be passed on-demand as args to the rewrite_response functions.
            optional_args_to_rewrite_function = {
                "request": self.request,
                "orig_response": original_response,
                "host": host,
                "port": port,
                "path": proxied_path,
            }

            # Initial value for rewriting
            rewritten_response = original_response

            for rewrite in rewrite_responses:
                # The rewrite function is a function of the RewritableResponse object
                # ``response`` as well as several other optional arguments. We need to
                # convert it to a function of only ``response`` by plugging in the
                # known values for all the other parameters. (This is called partial
                # evaluation.)
                def rewrite_pe(rewritable_response: RewritableResponse):
                    return call_with_asked_args(
                        rewrite,
                        {
                            "response": rewritable_response,
                            **optional_args_to_rewrite_function,
                        },
                    )

                # Now we can cleanly apply the partially evaulated function to a copy of
                # the rewritten response.
                rewritten_response = rewritten_response._apply_to_copy(rewrite_pe)

            # status
            self.set_status(rewritten_response.code, rewritten_response.reason)

            # clear tornado default header
            self._headers = httputil.HTTPHeaders()
            for header, v in rewritten_response.headers.get_all():
                if header not in ("Content-Length", "Transfer-Encoding", "Connection"):
                    # some header appear multiple times, eg 'Set-Cookie'
                    self.add_header(header, v)

            if rewritten_response.body:
                self.write(rewritten_response.body)

    async def proxy_open(self, host, port, proxied_path=""):
        """
        Called when a client opens a websocket connection.

        We establish a websocket connection to the proxied backend &
        set up a callback to relay messages through.
        """

        if not self._check_host_allowlist(host):
            self.set_status(403)
            self.log.info(
                "Host '{host}' is not allowed. "
                "See https://jupyter-server-proxy.readthedocs.io/en/latest/arbitrary-ports-hosts.html for info.".format(
                    host=host
                )
            )
            self.close()
            return

        if not proxied_path.startswith("/"):
            proxied_path = "/" + proxied_path

        if self.unix_socket is not None:
            assert host == "localhost", "Unix sockets only possible on localhost"
            self.log.debug("Opening websocket on Unix socket %r", port)
            resolver = UnixResolver(self.unix_socket)  # Requires tornado >= 6.3
        else:
            resolver = None

        client_uri = self.get_client_uri("ws", host, port, proxied_path)
        headers = self.proxy_request_headers()

        def message_cb(message):
            """
            Callback when the backend sends messages to us

            We just pass it back to the frontend
            """
            # Websockets support both string (utf-8) and binary data, so let's
            # make sure we signal that appropriately when proxying
            self._record_activity()
            if message is None:
                self.close()
            else:
                self.write_message(message, binary=isinstance(message, bytes))

        def ping_cb(data):
            """
            Callback when the backend sends pings to us.

            We just pass it back to the frontend.
            """
            self._record_activity()
            self.ping(data)

        async def start_websocket_connection():
            self.log.info(f"Trying to establish websocket connection to {client_uri}")
            self._record_activity()
            request = httpclient.HTTPRequest(url=client_uri, headers=headers)
            self.ws = await pingable_ws_connect(
                request=request,
                on_message_callback=message_cb,
                on_ping_callback=ping_cb,
                subprotocols=self._requested_subprotocols,
                resolver=resolver,
            )
            self._record_activity()
            self.log.info(f"Websocket connection established to {client_uri}")
            if self.ws.selected_subprotocol != self.selected_subprotocol:
                self.log.warn(
                    f"Websocket subprotocol between proxy/server ({self.ws.selected_subprotocol}) "
                    f"became different than for client/proxy ({self.selected_subprotocol}) "
                    "due to https://github.com/jupyterhub/jupyter-server-proxy/issues/459. "
                    f"Requested subprotocols were {self._requested_subprotocols}."
                )

        # Wait for the WebSocket to be connected before resolving.
        # Otherwise, messages sent by the client before the
        # WebSocket successful connection would be dropped.
        await start_websocket_connection()

    def proxy_request_headers(self):
        """A dictionary of headers to be used when constructing
        a tornado.httpclient.HTTPRequest instance for the proxy request."""
        headers = self.request.headers.copy()
        # Merge any manually configured request headers
        headers.update(self.get_request_headers_override())
        return headers

    def get_request_headers_override(self):
        """Add additional request headers. Typically overridden in subclasses."""
        return {}

    def proxy_request_options(self):
        """A dictionary of options to be used when constructing
        a tornado.httpclient.HTTPRequest instance for the proxy request."""
        return dict(
            follow_redirects=False, connect_timeout=250.0, request_timeout=300.0
        )

    def check_xsrf_cookie(self):
        """
        http://www.tornadoweb.org/en/stable/guide/security.html

        Defer to proxied apps.
        """

    def select_subprotocol(self, subprotocols):
        """
        Select a single Sec-WebSocket-Protocol during handshake.

        Overrides `tornado.websocket.WebSocketHandler.select_subprotocol` that
        includes an informative docstring:
        https://github.com/tornadoweb/tornado/blob/v6.4.0/tornado/websocket.py#L337-L360.
        """
        # Stash all requested subprotocols to be re-used as requested
        # subprotocols in the proxy/server handshake to be performed later. At
        # least bokeh has used additional subprotocols to pass credentials,
        # making this a required workaround for now.
        #
        self._requested_subprotocols = subprotocols if subprotocols else None

        if subprotocols:
            self.log.debug(
                f"Client sent subprotocols: {subprotocols}, selecting the first"
            )
            # FIXME: Subprotocol selection should be delegated to the server we
            #        proxy to, but we don't! For this to happen, we would need
            #        to delay accepting the handshake with the client until we
            #        have successfully handshaked with the server. This issue is
            #        tracked in https://github.com/jupyterhub/jupyter-server-proxy/issues/459.
            #
            return subprotocols[0]
        return None


class LocalProxyHandler(ProxyHandler):
    """
    A tornado request handler that proxies HTTP and websockets
    from a port on the local system. Same as the above ProxyHandler,
    but specific to 'localhost'.

    The arguments "port" and "proxied_path" in each method are extracted from
    the URL as capture groups in the regex specified in the add_handlers
    method.
    """

    async def http_get(self, port, proxied_path):
        return await self.proxy(port, proxied_path)

    async def open(self, port, proxied_path):
        return await self.proxy_open("localhost", port, proxied_path)

    def post(self, port, proxied_path):
        return self.proxy(port, proxied_path)

    def put(self, port, proxied_path):
        return self.proxy(port, proxied_path)

    def delete(self, port, proxied_path):
        return self.proxy(port, proxied_path)

    def head(self, port, proxied_path):
        return self.proxy(port, proxied_path)

    def patch(self, port, proxied_path):
        return self.proxy(port, proxied_path)

    def options(self, port, proxied_path):
        return self.proxy(port, proxied_path)

    def proxy(self, port, proxied_path):
        return super().proxy("localhost", port, proxied_path)


class RemoteProxyHandler(ProxyHandler):
    """
    A tornado request handler that proxies HTTP and websockets
    from a port on a specified remote system.

    The arguments "host", "port" and "proxied_path" in each method are
    extracted from the URL as capture groups in the regex specified in the
    add_handlers method.
    """

    async def http_get(self, host, port, proxied_path):
        return await self.proxy(host, port, proxied_path)

    def post(self, host, port, proxied_path):
        return self.proxy(host, port, proxied_path)

    def put(self, host, port, proxied_path):
        return self.proxy(host, port, proxied_path)

    def delete(self, host, port, proxied_path):
        return self.proxy(host, port, proxied_path)

    def head(self, host, port, proxied_path):
        return self.proxy(host, port, proxied_path)

    def patch(self, host, port, proxied_path):
        return self.proxy(host, port, proxied_path)

    def options(self, host, port, proxied_path):
        return self.proxy(host, port, proxied_path)

    async def open(self, host, port, proxied_path):
        return await self.proxy_open(host, port, proxied_path)

    def proxy(self, host, port, proxied_path):
        return super().proxy(host, port, proxied_path)


class NamedLocalProxyHandler(LocalProxyHandler):
    """
    A tornado request handler that proxies HTTP and websockets from a port on
    the local system. The port is specified in config, and associated with a
    name which forms part of the URL.

    Config will create a subclass of this for each named proxy. A further
    subclass below is used for named proxies where we also start the server.
    """

    port = 0
    mappath = {}

    @property
    def process_args(self):
        return {
            "port": self.port,
            "unix_socket": (self.unix_socket or ""),
            "base_url": self.base_url,
        }

    def _render_template(self, value):
        args = self.process_args
        if type(value) is str:
            return value.format(**args)
        elif type(value) is list:
            return [self._render_template(v) for v in value]
        elif type(value) is dict:
            return {
                self._render_template(k): self._render_template(v)
                for k, v in value.items()
            }
        else:
            raise ValueError(f"Value of unrecognized type {type(value)}")

    def _realize_rendered_template(self, attribute):
        """Call any callables, then render any templated values."""
        if callable(attribute):
            attribute = call_with_asked_args(attribute, self.process_args)
        return self._render_template(attribute)

    async def proxy(self, port, path):
        if not path.startswith("/"):
            path = "/" + path
        if self.mappath:
            if callable(self.mappath):
                path = call_with_asked_args(self.mappath, {"path": path})
            else:
                path = self.mappath.get(path, path)

        return await ensure_async(super().proxy(port, path))

    async def http_get(self, path):
        return await ensure_async(self.proxy(self.port, path))

    async def open(self, path):
        return await super().open(self.port, path)

    def post(self, path):
        return self.proxy(self.port, path)

    def put(self, path):
        return self.proxy(self.port, path)

    def delete(self, path):
        return self.proxy(self.port, path)

    def head(self, path):
        return self.proxy(self.port, path)

    def patch(self, path):
        return self.proxy(self.port, path)

    def options(self, path):
        return self.proxy(self.port, path)


# FIXME: Move this to its own file. Too many packages now import this from nbrserverproxy.handlers
class SuperviseAndProxyHandler(NamedLocalProxyHandler):
    """
    A tornado request handler that proxies HTTP and websockets from a local
    process which is launched on demand to handle requests. The command and
    other process options are specified in config.

    A subclass of this will be made for each configured server process.
    """

    def __init__(self, *args, **kwargs):
        self.requested_port = 0
        self.requested_unix_socket = False
        self.mappath = {}
        self.command = list()
        super().__init__(*args, **kwargs)

    def initialize(self, state):
        self.state = state
        if "proc_lock" not in state:
            state["proc_lock"] = Lock()

    name = "process"

    @property
    def port(self):
        """
        Allocate either the requested port or a random empty port for use by
        application
        """
        if self.requested_unix_socket:  # unix_socket has priority over port
            return 0

        if "port" not in self.state:
            if self.requested_port:
                self.state["port"] = self.requested_port
            else:
                sock = socket.socket()
                sock.bind(("", self.requested_port))
                self.state["port"] = sock.getsockname()[1]
                sock.close()

        return self.state["port"]

    @property
    def unix_socket(self):
        if "unix_socket" not in self.state:
            if self.requested_unix_socket is True:
                sock_dir = mkdtemp(prefix="jupyter-server-proxy-")
                sock_path = os.path.join(sock_dir, "socket")
            elif self.requested_unix_socket:
                sock_path = self.requested_unix_socket
            else:
                sock_path = None
            self.state["unix_socket"] = sock_path
        return self.state["unix_socket"]

    def get_cmd(self):
        return self._realize_rendered_template(self.command)

    def get_cwd(self):
        """Get the current working directory for our process

        Override in subclass to launch the process in a directory
        other than the current.
        """
        return os.getcwd()

    def get_env(self):
        """Set up extra environment variables for process. Typically
        overridden in subclasses."""
        return {}

    def get_timeout(self):
        """
        Return timeout (in s) to wait before giving up on process readiness
        """
        return 5

    async def _http_ready_func(self, p):
        if self.unix_socket is not None:
            url = "http://localhost"
            connector = aiohttp.UnixConnector(self.unix_socket)
        else:
            url = f"http://localhost:{self.port}"
            connector = None  # Default, TCP connector
        async with aiohttp.ClientSession(connector=connector) as session:
            try:
                async with session.get(url, allow_redirects=False) as resp:
                    # We only care if we get back *any* response, not just 200
                    # If there's an error response, that can be shown directly to the user
                    self.log.debug(f"Got code {resp.status} back from {url}")
                    return True
            except aiohttp.ClientConnectionError:
                self.log.debug(f"Connection to {url} refused")
                return False

    async def ensure_process(self):
        """
        Start the process
        """
        # We don't want multiple requests trying to start the process at the same time
        # FIXME: Make sure this times out properly?
        # Invariant here should be: when lock isn't being held, either 'proc' is in state &
        # running, or not.
        async with self.state["proc_lock"]:
            if "proc" not in self.state:
                # FIXME: Prevent races here
                # FIXME: Handle graceful exits of spawned processes here

                # When command option isn't truthy, it means its a process not
                # to be managed/started by jupyter-server-proxy. This means we
                # won't await its readiness or similar either.
                cmd = self.get_cmd()
                if not cmd:
                    self.state["proc"] = "process not managed by jupyter-server-proxy"
                    return

                # Set up extra environment variables for process
                server_env = os.environ.copy()
                server_env.update(self.get_env())

                timeout = self.get_timeout()

                proc = SupervisedProcess(
                    self.name,
                    *cmd,
                    env=server_env,
                    ready_func=self._http_ready_func,
                    ready_timeout=timeout,
                    log=self.log,
                )
                self.state["proc"] = proc

                try:
                    await proc.start()

                    is_ready = await proc.ready()

                    if not is_ready:
                        await proc.kill()
                        raise web.HTTPError(500, f"could not start {self.name} in time")
                except:
                    # Make sure we remove proc from state in any error condition
                    del self.state["proc"]
                    raise

    async def proxy(self, port, path):
        await self.ensure_process()
        return await ensure_async(super().proxy(port, path))

    async def open(self, path):
        await self.ensure_process()
        return await super().open(path)


def setup_handlers(web_app, serverproxy_config):
    host_allowlist = serverproxy_config.host_allowlist
    rewrite_response = serverproxy_config.non_service_rewrite_response
    web_app.add_handlers(
        ".*",
        [
            (
                url_path_join(
                    web_app.settings["base_url"],
                    r"/proxy/([^/:@]+):(\d+)(/.*|)",
                ),
                RemoteProxyHandler,
                {
                    "absolute_url": False,
                    "host_allowlist": host_allowlist,
                    "rewrite_response": rewrite_response,
                },
            ),
            (
                url_path_join(
                    web_app.settings["base_url"],
                    r"/proxy/absolute/([^/:@]+):(\d+)(/.*|)",
                ),
                RemoteProxyHandler,
                {
                    "absolute_url": True,
                    "host_allowlist": host_allowlist,
                    "rewrite_response": rewrite_response,
                },
            ),
            (
                url_path_join(
                    web_app.settings["base_url"],
                    r"/proxy/(\d+)(/.*|)",
                ),
                LocalProxyHandler,
                {
                    "absolute_url": False,
                    "rewrite_response": rewrite_response,
                },
            ),
            (
                url_path_join(
                    web_app.settings["base_url"],
                    r"/proxy/absolute/(\d+)(/.*|)",
                ),
                LocalProxyHandler,
                {
                    "absolute_url": True,
                    "rewrite_response": rewrite_response,
                },
            ),
        ],
    )
