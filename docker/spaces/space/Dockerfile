ARG BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda11.8.0-1.0.4
FROM ${BASE_IMAGE} 
USER root

RUN apt update && \
    apt install -y --no-install-recommends \
    curl \
    nginx && \
    pip install --upgrade pip uv --no-cache-dir && \
    apt-get clean autoclean && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
    
RUN mkdir -p /var/log/nginx /var/lib/nginx /var/cache/nginx /home/user/app/logs/nginx /home/user/app/nginx/run && \
    chown -R user:user /var/log/nginx /var/lib/nginx /var/cache/nginx /home/user/app/logs/nginx /home/user/app/nginx/run && \
    chmod -R 755 /etc/nginx && \
    chmod -R 755 /usr/share/nginx/html

RUN chown -R user:user /home/user
USER user
WORKDIR /home/user/app

RUN pip install --user -i https://pypi.tuna.tsinghua.edu.cn/simple csghub-sdk  gradio streamlit

COPY --chown=user:user ./start.sh /home/user/app/

ENV PYTHONUNBUFFERED=1 \
    GRADIO_ALLOW_FLAGGING=never\
    GRADIO_SERVER_NAME=0.0.0.0

EXPOSE 8000
EXPOSE 7860
CMD ["bash", "/home/user/app/start.sh"]