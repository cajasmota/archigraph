ARG BUILD_PLATFORM=linux/amd64

FROM --platform=$BUILD_PLATFORM ortgenai:latest AS ortgenai-test

COPY . /build

RUN cd /build && \
    chown -R testuser:testuser /build && \
    curl -LO https://github.com/gotestyourself/gotestsum/releases/download/v1.13.0/gotestsum_1.13.0_linux_amd64.tar.gz && \
    tar -xzf gotestsum_1.13.0_linux_amd64.tar.gz --directory /usr/local/bin && \
    # entrypoint
    cp /build/scripts/entrypoint.sh /entrypoint.sh && sed -i 's/\r//g' /entrypoint.sh && chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

#--- artifacts layer ---
FROM --platform=$BUILD_PLATFORM scratch AS artifacts
COPY --from=ortgenai-test /usr/lib/libonnxruntime-genai.so libonnxruntime-genai-linux-x64.so
