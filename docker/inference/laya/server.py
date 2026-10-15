import argparse
import logging
import os

import uvicorn
from laya import Router
from laya.serve import create_app

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)


def build_models(base):
    models = {"english": base}
    for name in ("multilingual", "typed-decisions"):
        sub = os.path.join(base, name)
        if os.path.isdir(sub):
            models[name] = sub
    return models


def main():
    parser = argparse.ArgumentParser(description="Laya System-1 decision API server")
    parser.add_argument("--host", default="0.0.0.0", help="Bind host")
    parser.add_argument("--port", type=int, default=8000, help="Bind port")
    args, unknown_args = parser.parse_known_args()
    if unknown_args:
        logger.warning("Ignoring unsupported server args: %s", unknown_args)

    base = os.path.join("/workspace", os.environ["REPO_ID"])
    models = build_models(base)
    logger.info("Loading laya models: %s", list(models.keys()))
    router = Router(models=models)
    router.preload(list(models.keys()))
    app = create_app(router)

    logger.info("Laya API server starting on http://%s:%s", args.host, args.port)
    uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
