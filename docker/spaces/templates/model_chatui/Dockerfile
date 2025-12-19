ARG SPACE_BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com
ARG NAMESPACE=opencsghq
FROM ${SPACE_BASE_IMAGE}/${NAMESPACE}/vllm-cpu-chatui-base:1.0

WORKDIR /workspace/

COPY ./*.jinja /etc/csghub/
COPY ./*.py /etc/csghub/
COPY ./*.sh /etc/csghub/
RUN chmod +x /etc/csghub/*.sh

ENV HUGGINGFACE_HUB_CACHE=/workspace/ \
    HF_HUB_ENABLE_HF_TRANSFER=0 \
    HF_HUB_OFFLINE=1 \
    ENABLE_EVALUATION_ARENA_MODELS=false \
    ENABLE_OLLAMA_API=false \
    ENABLE_OPENAI_API=true \
    OPENAI_API_BASE_URL=http://127.0.0.1:11231/v1

EXPOSE 8080

ENTRYPOINT ["bash", "/etc/csghub/serve-cpu.sh"]
