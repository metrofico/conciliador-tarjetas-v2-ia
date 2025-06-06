SHELL := /bin/bash

# Repositorio base
REGISTRY := kfcregistry.azurecr.io/conciliator-v2

.PHONY: colores

# Lista de servicios que quieres construir (dentro de providers/)
SERVICES := api-central \
			report-system \
			sir-writer \
			kioscos-services \
			datafast-services \
			invoker-services

# Construir un servicio individual: make build SERVICE=api-central
build:
	@if "$(SERVICE)"=="" ( \
		echo Error: Debes pasar el nombre del servicio con SERVICE=nombre & \
		exit /b 1 \
	)
	@echo Construyendo $(SERVICE)...
	@if exist providers\$(SERVICE)\Dockerfile ( \
		docker build -f providers/$(SERVICE)/Dockerfile -t $(REGISTRY)/$(SERVICE):latest . \
	) else ( \
		echo No se encontro Dockerfile en providers/$(SERVICE) & \
		exit /b 1 \
	)

# Subir un servicio individual: make push SERVICE=api-central
push:
	@if "$(SERVICE)"=="" ( \
		echo Error: Debes pasar el nombre del servicio con SERVICE=nombre & \
		exit /b 1 \
	)
	@echo Subiendo $(SERVICE)...
	@docker push $(REGISTRY)/$(SERVICE):latest

# Construir todos los servicios definidos en SERVICES
all-build:
	@for %%s in ($(SERVICES)) do ( \
		echo [*] Construyendo %%s... & \
		if exist providers\%%s\Dockerfile ( \
			docker build -f providers/%%s/Dockerfile -t $(REGISTRY)/%%s:latest . || exit /b 1 \
		) else ( \
			echo No se encontro Dockerfile en providers/%%s & \
			exit /b 1 \
		) \
	)

# Subir todos los servicios definidos en SERVICES
all-push:
	@for %%s in ($(SERVICES)) do ( \
		echo [*] Subiendo %%s... & \
		docker push $(REGISTRY)/%%s:latest || exit /b 1 \
	)

install:
	helm install conciliador-tarjetas-v2 -n conciliador-tarjetas-v2 --create-namespace .\conciliador

update:
	helm upgrade conciliador-tarjetas-v2 -n conciliador-tarjetas-v2 ./conciliador/