# Contexto de build = raiz do lab (labs/instrumentation).
# SRC escolhe de onde vem o app.py: app-python (enunciado) ou solutions/app-python (gabarito).
FROM python:3.14-alpine
ARG SRC=app-python
WORKDIR /app
COPY app-python/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY ${SRC}/app.py ./
USER 65534
EXPOSE 8080
ENTRYPOINT ["python", "-u", "app.py"]
