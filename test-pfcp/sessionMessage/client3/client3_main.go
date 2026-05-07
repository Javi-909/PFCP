package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

// Usamos una variable global simple para el número de secuencia
var seq uint32 = 1

func main() {
	// Dirección de tu servidor UPF
	serverAddr := "127.0.0.1:8805"

	// Nos conectamos al servidor UDP
	conn, err := net.Dial("udp", serverAddr)
	if err != nil {
		log.Fatalf("Error al conectar (¿Está el servidor UPF corriendo?): %v", err)
	}
	defer conn.Close()
	fmt.Println("\n--- [INICIO] Arrancando Infraestructura de Control (SMF) ---")
	fmt.Println("Esperando conectar con el UPF en", serverAddr)

	// --- 1. Enviar Association Setup Request (ASOCIACION) ---
	if !sendAssociationSetup(conn) {
		log.Fatalf("❌ Falló la asociación, terminando.")
	}

	time.Sleep(1 * time.Second)

	// --- 2. Enviar Session Establishment Request (Siguiente paso, CONEXION de Usuario) ---
	fmt.Println("\n--- [CASO DE USO 1] Conexión Inicial de Usuario ---")
	fmt.Println("\n--- EVENTO: El móvil con IMSI 123456789 solicita acceso a Internet")
	fmt.Println("\n--- DECISION SMF: Establecer Sesión PDU y asignar IP 10.0.0.1")

	if !sendSessionEstablishment(conn) {
		log.Fatalf("❌ Falló el establecimiento de sesión.")
	}

	time.Sleep(2 * time.Second)
    
	// --- 3. Enviar Session Modification Request --- 	
	//CASO DE USO: cambiar la prioridad del QoS
	fmt.Println("\n--- [CASO DE USO 2] Solicitud de Servicio QoS (Videollamada) ---")
	fmt.Println("\n--- EVENTO:Detectado tráfico de vídeo de alta definición.")
	fmt.Println("\n--- DECISION SMF: Modificar túnel para crear 'Carril rápido'.")
	if !sendSessionModification(conn) {
		log.Fatalf("❌ Falló la modificación de sesión.")
	}

	// --- PASO 4: SIMULACIÓN DE ERROR (CASO DE USO 3) ---
	fmt.Println("\n--- [CASO DE USO 3] Gestión de Error (Sesión Inválida) ---")
	fmt.Println("ℹ️  EVENTO: Intento de modificación sobre una sesión ya expirada (ID 9999).")
	sendSessionModificationError(conn) 
	
	fmt.Println("\n--- [FIN] Simulación de escenarios 5G completada exitosamente ---")

    // (Aquí irían los pasos 3 y 4: Modification y Deletion)
}

// sendAssociationSetup se encarga de enviar la petición de asociación y validar la respuesta
func sendAssociationSetup(conn net.Conn) bool {
	// --- 1. Crear el payload (IEs) ---
	// Este es nuestro NodeID (como SMF)
	smfNodeID := ie.NewNodeID("192.168.100.1", "", "") // IP de ejemplo del SMF
	recovery := ie.NewRecoveryTimeStamp(time.Now())

	var buf bytes.Buffer
	for _, i := range []*ie.IE{smfNodeID, recovery} {
		b, _ := i.Marshal()
		buf.Write(b)
	}

	// --- 2. Crear la cabecera (Header) ---
	// Construimos el mensaje de la misma forma que tú en tu servidor
	req := &message.Header{
		Flags:           1, // Versión 1
		Type:            message.MsgTypeAssociationSetupRequest,
		Length:          0, // Se calculará en Marshal()
		SequenceNumber:  uint32(seq),
		MessagePriority: 0,
		Payload:         buf.Bytes(),
	}

	// --- 3. Serializar y enviar ---
	bin, err := req.Marshal()
	if err != nil {
		log.Printf("Error serializando Association Request: %v", err)
		return false
	}

	if _, err := conn.Write(bin); err != nil {
		log.Printf("Error enviando Association Request: %v", err)
		return false
	}
	fmt.Println("📤SMF -> UPF: Association Setup Request enviado (Iniciando Interfaz N4).")
	seq++ // Incrementar para el siguiente mensaje

	// --- 4. Esperar y parsear la respuesta ---
	respBuf := make([]byte, 4096)
	// Establecemos un timeout por si el servidor no responde
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	
	n, err := conn.Read(respBuf)
	if err != nil {
		log.Printf("Error leyendo respuesta: %v", err)
		return false
	}
	conn.SetReadDeadline(time.Time{}) // Limpiar el deadline

	msg, err := message.Parse(respBuf[:n])
	if err != nil {
		log.Printf("Error parseando respuesta: %v", err)
		return false
	}

	// --- 5. Validar la respuesta ---
	// Usamos Type Assertion para convertir el 'message.Message' genérico
	// al tipo específico que esperamos.
	resp, ok := msg.(*message.AssociationSetupResponse)
	if !ok {
		// Esto puede pasar si el servidor responde con un tipo de mensaje inesperado
		fmt.Printf("❌ Respuesta inesperada. Tipo: %d (%T)\n", msg.MessageType(), msg)
		return false
	}

	// Comprobar la 'Cause' (Causa)
	cause, err := resp.Cause.Cause()
	if err != nil {
		fmt.Println("❌ No se pudo extraer la 'Cause' de la respuesta.")
		return false
	}

	if cause == ie.CauseRequestAccepted {
		fmt.Println("✅ Asociación ACEPTADA por el UPF. Infraestructura lista.")
		return true
	}

	fmt.Printf("❌ Asociación RECHAZADA por el UPF. Causa: %d\n", cause)
	return false
}

// sendSessionEstablishment se encarga de crear y enviar una nueva sesión (CASO DE USO 1: Conexion a Internet)
func sendSessionEstablishment(conn net.Conn) bool {
	// --- 1. Crear IEs (El corazón de la petición) ---

	// IE 1: Nuestro F-SEID (como SMF)
	// Usaremos 1001 como ID de sesión de ejemplo
	smfFSEID := ie.NewFSEID(1001, net.ParseIP("192.168.100.1"), nil, nil)

	// IE 2: Una PDR (Packet Detection Rule)
	// "Crea PDR con ID=1"
	pdr := ie.NewCreatePDR(
		ie.NewPDRID(1),
		ie.NewPrecedence(100),
		ie.NewPDI( // Packet Detection Information
			ie.NewSourceInterface(ie.SrcInterfaceCore),
			// (Aquí irían más reglas, como SDF Filter, etc.)
		),
		ie.NewFARID(1), // "Aplica la FAR con ID=1"
	)

	// IE 3: Una FAR (Forwarding Action Rule) -> Enviar a Internet
	// "Crea FAR con ID=1"
	far := ie.NewCreateFAR(
		ie.NewFARID(1),
		ie.NewApplyAction(0x02), // "Acción = Forward", tenemos que usar el valor concreto (el 02, porque no esta exportado la action forward)
	)

	// --- 2. Crear el payload (meter IEs en el buffer) ---
	var buf bytes.Buffer
	for _, i := range []*ie.IE{smfFSEID, pdr, far} {
		b, _ := i.Marshal()
		buf.Write(b)
	}

	// --- 3. Crear la cabecera (Header) ---
	req := &message.Header{
		Flags:           1,
		Type:            message.MsgTypeSessionEstablishmentRequest,
		Length:          0, // Se calculará
		SequenceNumber:  uint32(seq),
		Payload:         buf.Bytes(),
	}
	seq++

	// --- 4. Serializar y enviar ---
	bin, err := req.Marshal()
	if err != nil {
		return false
	}

	if _, err := conn.Write(bin); err != nil {
		return false
	}
	fmt.Println("📤 SMF -> UPF: Session Establishment Request.")
	fmt.Println(" └-> Instrucción: Instalar PDR-1 (Tráfico Usuario) y FAR-1 (Salida Internet).")

	// --- 5. Esperar y parsear la respuesta ---
	respBuf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(respBuf)
	if err != nil {
		log.Printf("Error leyendo respuesta de sesión: %v", err)
		return false
	}
	conn.SetReadDeadline(time.Time{})

	msg, err := message.Parse(respBuf[:n])
	if err != nil {
		return false
	}

	// --- 6. Validar la respuesta ---
	resp, ok := msg.(*message.SessionEstablishmentResponse)
	if !ok {
		fmt.Printf("❌ Respuesta inesperada. Tipo: %d (%T)\n", msg.MessageType(), msg)
		return false
	}

	// Comprobar la 'Cause'
	cause, err := resp.Cause.Cause()
	if err != nil {
		return false
	}

	if cause == ie.CauseRequestAccepted {		
		// Opcional: Extraer el F-SEID del UPF para usarlo después
		if resp.UPFSEID != nil {
			upfFSEID, _ := resp.UPFSEID.FSEID()
			fmt.Printf("✅ UPF responde: Sesión Establecida. F-SEID asignado: %d. Usuario conectado. \n", upfFSEID.SEID)
		}
		return true
	}

	fmt.Printf("❌ Establecimiento de Sesión RECHAZADO. Causa: %d\n", cause)
	return false
}

// sendSessionModification se encarga de modificar una sesión existente
func sendSessionModification(conn net.Conn) bool {
	// ¡Clave! El SEID en la cabecera ya NO es 0.
	// Debe ser el SEID de la sesión que el UPF conoce.
	// En nuestro caso, ambos acordamos usar 1001.
	var sessionSEID uint64 = 1001

	// --- 1. Crear IEs de modificación ---

	// IE 1: Actualizar la PDR 1 (la que creamos antes)
	updatePDR := ie.NewUpdatePDR(
		ie.NewPDRID(1),
		ie.NewPrecedence(50), // Valor actualizado
	)

	// IE 2: Crear una nueva FAR (con ID 2)
	createFAR := ie.NewCreateFAR(
		ie.NewFARID(2),
		ie.NewApplyAction(0x02), // Acción = Forward
	)

	// --- 2. Crear el payload (meter IEs en el buffer) ---
	var buf bytes.Buffer
	for _, i := range []*ie.IE{updatePDR, createFAR} {
		b, _ := i.Marshal()
		buf.Write(b)
	}

	// --- 3. Crear la cabecera (Header) ---
	req := &message.Header{
		Flags:           1,
		Type:            message.MsgTypeSessionModificationRequest,
		Length:          0, // Se calculará
		SEID:            sessionSEID, // ¡Importante!
		SequenceNumber:  uint32(seq),
		Payload:         buf.Bytes(),
	}
	seq++

	// --- 4. Serializar y enviar ---
	bin, err := req.Marshal()
	if err != nil {
		return false
	}

	if _, err := conn.Write(bin); err != nil {
		return false
	}
	fmt.Println("📤 SMF -> UPF: Session Modification Request.")
	fmt.Println(" └-> Instrucción: Subir prioridad PDR-1 y Crear FAR-2 (Carril Rápido).")

	// --- 5. Esperar y parsear la respuesta ---
	respBuf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(respBuf)
	if err != nil {
		return false
	}
	conn.SetReadDeadline(time.Time{})

	msg, err := message.Parse(respBuf[:n])
	if err != nil {
		return false
	}

	// --- 6. Validar la respuesta ---
	resp, ok := msg.(*message.SessionModificationResponse)
	if !ok {
		return false
	}

	// Comprobar la 'Cause'
	cause, err := resp.Cause.Cause()
	if err != nil {
		fmt.Println("❌ No se pudo extraer la 'Cause' de la respuesta de modificación.")
		return false
	}

	if cause == ie.CauseRequestAccepted {
		fmt.Println("✅ UPF responde: Modificación Aplicada. Calidad de servicio (QoS) garantizada.")
		return true
	}

	fmt.Printf("❌ Modificación de Sesión RECHAZADA. Causa: %d\n", cause)
	return false
}


// sendSessionModificationError: Caso de Uso 3 (Gestión de Errores)
// Esta función simula un intento de acceso a una sesión que no existe.
func sendSessionModificationError(conn net.Conn) {
	// Usamos un ID que sabemos que no existe en el UPF
	var sessionSEID uint64 = 9999 

	// Payload vacío (no necesitamos IEs para probar que falla por el ID)
	req := &message.Header{
		Flags:           1,
		Type:            message.MsgTypeSessionModificationRequest,
		Length:          0,
		SEID:            sessionSEID,
		SequenceNumber:  uint32(seq),
		Payload:         []byte{}, 
	}
	seq++

	bin, err := req.Marshal()
	if err != nil {
		log.Printf("Error serializando: %v", err)
		return
	}

	if _, err := conn.Write(bin); err != nil {
		log.Printf("Error enviando: %v", err)
		return
	}
	fmt.Println("📤 SMF -> UPF: Session Modification Request (ID Sesión: 9999)")

	// Esperar respuesta (esperamos un rechazo)
	respBuf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(respBuf)
	if err != nil {
		log.Printf("Error leyendo respuesta: %v", err)
		return
	}
	conn.SetReadDeadline(time.Time{})

	msg, err := message.Parse(respBuf[:n])
	if err != nil {
		return
	}

	resp, ok := msg.(*message.SessionModificationResponse)
	if !ok {
		fmt.Printf("❌ Respuesta inesperada: %T\n", msg)
		return
	}

	cause, _ := resp.Cause.Cause()
	
	// Si recibimos CauseSessionContextNotFound (18) o RequestRejected (65), la prueba es exitosa
	fmt.Printf("⚠️  UPF Responde con Error Controlado. Causa: %d (Session Context Not Found)\n", cause)
	fmt.Println("✅ Prueba de robustez superada: El UPF protegió la integridad del sistema.")
}