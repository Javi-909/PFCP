# Demostrador de Redes SDN basadas en el Protocolo PFCP (5G Core)

Este repositorio contiene el código fuente del Trabajo Fin de Grado desarrollado en la ETSIT-UPM titulado: "Desarrollo y Análisis de Redes SDN Basadas en PFCP". El proyecto consiste en un demostrador funcional que emula la separación entre el plano de control (SMF) y el plano de usuario (UPF) mediante el protocolo PFCP (3GPP TS 29.244).
## 🚀 Resumen del Proyecto

El sistema permite orquestar sesiones de usuario de forma dinámica, abstrayendo la complejidad binaria del protocolo mediante una interfaz web interactiva. Los componentes principales son:

* **SMF (Session Management Function):** Actúa como controlador SDN. Expone una API REST para recibir directrices del operador y las traduce en mensajes PFCP binarios.
* **UPF (User Plane Function):** Emula el procesamiento de tráfico. Implementa un motor de reglas (PDR, FAR, QER) y gestiona el estado de las sesiones de forma concurrente mediante *Goroutines* y mecanismos de exclusión mutua (*RWMutex*).
* **Interfaz de Control Web:** Dashboard desarrollado en HTML5/JS que permite la inyección de reglas y la monitorización de logs en tiempo real.

## 🛠️ Requisitos del Sistema

Para la ejecución del demostrador es necesario contar con:
1.  **Lenguaje Go:** Versión 1.18 o superior.
2.  **Git:** Para la clonación del repositorio.
3.  **Navegador Web:** Chrome, Firefox o similares para acceder al panel de control.
4.  **Wireshark (Opcional):** Recomendado para la inspección de tramas en la interfaz de loopback (puerto 8805).

## 💻 Instrucciones de Instalación y Ejecución

Sigue estos pasos para desplegar el entorno en tu equipo local:

1.  **Clonar el repositorio:**
    ```bash
    git clone [https://github.com/Javi-909/PFCP.git](https://github.com/Javi-909/PFCP.git)
    cd PFCP
    ```

2.  **Iniciar el Plano de Usuario (UPF):**
    Abre una terminal, accede a la carpeta del UPF y ejecuta el proceso. Este quedará a la escucha de señalización en el puerto UDP 8805.
    ```bash
    cd upf
    go run .
    ```

3.  **Iniciar el Plano de Control (SMF):**
    Abre una segunda terminal, accede a la carpeta del SMF y ejecuta el proceso. Este iniciará el servidor web en el puerto 8080.
    ```bash
    cd smf
    go run .
    ```

4.  **Acceder al Orquestador:**
    Abre tu navegador y dirígete a: `http://localhost:8080`

## 📂 Estructura del Repositorio

* `smf/`: Contiene la lógica del plano de control, la API REST y el cliente PFCP, además de archivos del frontend (index.html).
* `upf/`: Contiene la implementación del plano de usuario, el servidor PFCP y el motor de reglas.
* `go.mod` / `go.sum`: Archivos de gestión de dependencias del proyecto.

---
**Autor:** Javier Quevedo Santos   
**Año:** 2026
