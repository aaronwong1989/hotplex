# ============================================
# HotPlex Primary Image (Standard Stack)
# ============================================
# This image inherits from the shared base foundation.
# Base includes: OS, Common Tools, Go Toolchain, Claude Code, and hotplexd binary.
# ============================================

ARG BASE_IMAGE=hotplex:base
FROM ${BASE_IMAGE}

LABEL org.opencontainers.image.title="HotPlex"
LABEL org.opencontainers.image.description="Standard production environment for HotPlex AI Agent"

# Hierarchy: Dockerfile.base -> Dockerfile (this)
