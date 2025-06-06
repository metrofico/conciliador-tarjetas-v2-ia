FROM golang:1.24 AS builder
RUN rm -rf /app

RUN mkdir /app

WORKDIR /app
COPY ./providers/datafast-services /app/datafast-services
COPY ./lib-shared /lib-shared

WORKDIR /app/datafast-services
# Compilar la aplicación
RUN CGO_ENABLED=0 go build -o myapp ./cmd/main.go

# Etapa 2: Crear una imagen mínima
FROM alpine:latest
RUN apk add --no-cache tzdata
ENV TZ=America/Bogota
WORKDIR /app

# Copiar solo los archivos necesarios desde la etapa de construcción
COPY --from=builder /app/datafast-services/myapp /app/myapp

# Comando para ejecutar la aplicación
CMD ["./myapp"]