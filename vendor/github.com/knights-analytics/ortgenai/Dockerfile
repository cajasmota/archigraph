#--- dockerfile to test ortgenai ---

ARG GO_VERSION=1.26.2
ARG ONNXRUNTIME_VERSION=1.25.0
ARG ONNXRUNTIME_GENAI_VERSION=0.13.1
ARG BUILD_PLATFORM=linux/amd64

FROM --platform=$BUILD_PLATFORM public.ecr.aws/amazonlinux/amazonlinux:2023 AS genai-runtime
ARG GO_VERSION
ARG ONNXRUNTIME_VERSION
ARG ONNXRUNTIME_GENAI_VERSION

ENV PATH="$PATH:/usr/local/go/bin"

COPY ./scripts/download-onnxruntime.sh /download-onnxruntime.sh
COPY ./scripts/download-onnxruntime-genai.sh /download-onnxruntime-genai.sh
RUN --mount=src=./go.mod,dst=/go.mod \
    dnf --allowerasing -y install gcc jq bash tar xz gzip glibc-static libstdc++ wget zip git dirmngr sudo which && \
    ln -s /usr/lib64/libstdc++.so.6 /usr/lib64/libstdc++.so && \
    dnf clean all && \
    # go
    curl -LO https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz && \
    rm go${GO_VERSION}.linux-amd64.tar.gz && \
    # onnxruntime cpu
    sed -i 's/\r//g' /download-onnxruntime.sh && chmod +x /download-onnxruntime.sh && \
    /download-onnxruntime.sh ${ONNXRUNTIME_VERSION} && \
    # onnxruntime genai
    sed -i 's/\r//g' /download-onnxruntime-genai.sh && chmod +x /download-onnxruntime-genai.sh && \
    /download-onnxruntime-genai.sh ${ONNXRUNTIME_GENAI_VERSION} && \
    # NON-PRIVILEGED USER
    # create non-privileged testuser with id: 1000
    useradd -u 1000 -m testuser && usermod -a -G wheel testuser && \
    echo "testuser ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers.d/testuser

COPY . /build
RUN cd /build && chown -R testuser:testuser /build
