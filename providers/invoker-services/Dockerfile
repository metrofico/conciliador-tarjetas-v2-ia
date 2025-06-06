FROM golang:1.24 AS builder
RUN rm -rf /app

RUN mkdir /app

WORKDIR /app
COPY ./providers/invoker-services /app/invoker-services
COPY ./lib-shared /lib-shared

WORKDIR /app/invoker-services
# Compilar la aplicación
RUN CGO_ENABLED=0 go build -o myapp ./cmd/main.go

# Etapa 2: Crear una imagen mínima
FROM alpine:latest
RUN apk add --no-cache tzdata
ENV TZ=America/Bogota
WORKDIR /app

# Copiar solo los archivos necesarios desde la etapa de construcción
COPY --from=builder /app/invoker-services/myapp /app/myapp

# Comando para ejecutar la aplicación
CMD ["./myapp"]