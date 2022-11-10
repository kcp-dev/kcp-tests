# Copyright 2022 The KCP Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the binary
FROM golang:1.18 AS builder

WORKDIR /build-dir

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Cache dependencies before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
USER 0
RUN go mod download

# Copy the go source 
COPY Makefile Makefile
COPY bindata.mk bindata.mk
COPY pkg/ pkg/
COPY cmd/ cmd/
COPY hack/ hack/
COPY test/ test/

# Build the kcp-tests binary
RUN make build

FROM quay.io/centos/centos:stream8
LABEL maintainer="KCP QE Team"
USER root
WORKDIR /

RUN dnf install -y unzip jq && \
    dnf clean all && \
    curl -fsL -o /usr/local/bin/kubectl "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/$(uname -m | sed 's/aarch.*/arm64/;s/armv8.*/arm64/;s/x86_64/amd64/')/kubectl" && \   
    curl -fsL -o /tmp/kubelogin.zip "https://github.com/int128/kubelogin/releases/download/$(curl -L -s https://dl.k8s.io/release/stable.txt)/kubelogin_linux_$(uname -m | sed 's/aarch.*/arm64/;s/armv8.*/arm64/;s/x86_64/amd64/').zip" && \
    unzip -od /tmp /tmp/kubelogin.zip && \
    mv /tmp/kubelogin /usr/local/bin/kubectl-oidc_login && \
    # Download the latest release kcp plugins packages by default
    curl -fsL -o /tmp/kubectl-kcp-plugins.tar.gz "https://github.com/kcp-dev/kcp/releases/download/$(curl "https://api.github.com/repos/kcp-dev/kcp/releases/latest" | jq -r .tag_name)/kubectl-kcp-plugin_$(curl "https://api.github.com/repos/kcp-dev/kcp/releases/latest" | jq -r .tag_name | sed -e "s/v//")_linux_$(uname -m | sed 's/aarch.*/arm64/;s/armv8.*/arm64/;s/x86_64/amd64/').tar.gz" && \
    tar -zxvf /tmp/kubectl-kcp-plugins.tar.gz -C /tmp && \
    cp /tmp/bin/kubectl-* /usr/local/bin/ && \
    # Download from release already contains the symbolic
    # ln -s /usr/local/bin/kubectl-workspace /usr/local/bin/kubectl-workspaces && ln -s /usr/local/bin/kubectl-workspace /usr/local/bin/kubectl-ws && \
    chmod +x /usr/local/bin/* && \
    rm -rf /tmp/*
    
COPY --from=builder build-dir/bin/kcp-tests /usr/local/bin/
RUN chmod +x /usr/local/bin/kcp-tests
