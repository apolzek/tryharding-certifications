FROM python:3.14-alpine
WORKDIR /app
COPY generator.py .
USER nobody
EXPOSE 8000
HEALTHCHECK --interval=2s --timeout=2s --retries=15 CMD wget -qO- http://127.0.0.1:8000/healthz || exit 1
CMD ["python", "-u", "generator.py"]
