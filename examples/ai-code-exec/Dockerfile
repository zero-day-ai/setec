# SPDX-License-Identifier: Apache-2.0
#
# Minimal Python image for the ai-code-exec example workload. The client
# program pipes source code via `python3 -c`; this image exists so users
# can pin a known Python version rather than pulling from Docker Hub at
# launch time.

FROM python:3.12-slim

# Harden: run as a non-root user inside the VM.
RUN useradd --create-home --shell /usr/sbin/nologin runner
USER runner
WORKDIR /home/runner

# No ENTRYPOINT on purpose. The Sandbox spec supplies the command.
CMD ["python3", "--version"]
