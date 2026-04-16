# SPDX-License-Identifier: Apache-2.0
#
# Minimal Node.js 20 image for the ci-sandbox example workload.
# The client program injects the project as a tarball at launch time.

FROM node:20-slim

# The sandbox extracts the workspace into /workspace at runtime.
WORKDIR /workspace

# Harden: run as a non-root user.
RUN useradd --create-home --shell /usr/sbin/nologin runner \
 && chown runner:runner /workspace
USER runner

CMD ["node", "--version"]
