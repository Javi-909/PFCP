package main

import (
	"encoding/json"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

var seidCounter uint64 = 1000

type LogMessage struct {
	Message string `json:"message"`
}

var (
	upfLogsMu sync.Mutex
	upfLogs   []string
)

type Marshaler interface {
	Marshal() ([]byte, error)
}

func main() {
	fs := http.FileServer(http.Dir("./"))
	http.Handle("/", fs)

	http.HandleFunc("/api/associate", handleAssociate)
	http.HandleFunc("/api/establish", handleEstablish)
	http.HandleFunc("/api/modify", handleModify)
	http.HandleFunc("/api/delete", handleDelete)
	http.HandleFunc("/api/upf-log", handleUpfLog)
	http.HandleFunc("/api/get-logs", getLogs)

	fmt.Println("--------------------------------------------------")
	fmt.Println(" SMF CONTROL PANEL: http://127.0.0.1:8080")
	fmt.Println(" Modo: Sandbox Parametrizable Avanzado (FAR/QER)")
	fmt.Println("--------------------------------------------------")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleAssociate(w http.ResponseWriter, r *http.Request) {
	nodeIP := r.URL.Query().Get("node")
	if nodeIP == "" { nodeIP = "127.0.0.1" }

	fmt.Printf("\n[SMF] >>> Ejecutando: ASSOCIATE hacia nodo %s...\n", nodeIP)
	req := message.NewAssociationSetupRequest(1, ie.NewNodeID("", "", nodeIP), ie.NewRecoveryTimeStamp(time.Now()))
	
	if err := enviarMensajePFCP(req, nodeIP); err != nil {
		responderError(w, fmt.Sprintf("Error: UPF en %s no alcanzable", nodeIP))
		return
	}
	responderExito(w, fmt.Sprintf("Asociación N4 con UPF (%s) establecida.", nodeIP), 0)
}

func handleEstablish(w http.ResponseWriter, r *http.Request) {
	seidParam := r.URL.Query().Get("id")
	ueIP := r.URL.Query().Get("ip")
	
	var nuevoSeid uint64
	if seidParam != "" {
		nuevoSeid, _ = strconv.ParseUint(seidParam, 10, 64)
	} else {
		nuevoSeid = atomic.AddUint64(&seidCounter, 1)
	}

	if ueIP == "" { ueIP = "192.168.100.1" }

	fmt.Printf("\n[SMF] >>> Ejecutando: ESTABLISH para SEID %d (IP UE: %s)...\n", nuevoSeid, ueIP)
	
	// Por defecto al crear, permitimos tráfico (FORW).
	est := message.NewSessionEstablishmentRequest(
		0, 0, 0, uint32(1), 0,
		ie.NewNodeID("", "", "127.0.0.1"),
		ie.NewFSEID(nuevoSeid, net.ParseIP("127.0.0.1"), nil, nil),
		ie.NewCreatePDR(
			ie.NewPDRID(1),
			ie.NewPrecedence(100),
			ie.NewPDI(
				ie.NewSourceInterface(ie.SrcInterfaceAccess),
				ie.NewFTEID(uint32(nuevoSeid), net.ParseIP("127.0.0.1"), nil, nil),
				ie.NewUEIPAddress(2, ueIP, "", 0), 
			),
		),
		ie.NewCreateFAR(ie.NewFARID(1), ie.NewApplyAction(0x02)), // 0x02 = FORW
	)
	
	if err := enviarMensajePFCP(est, "127.0.0.1"); err != nil {
		responderError(w, "Fallo al establecer sesión de usuario.")
		return
	}
	
	mensajeFront := fmt.Sprintf("\n--- [CREACIÓN DE TÚNEL] ---\nℹ️ Evento: Móvil (%s) solicita acceso.\n📤 [SMF] Enviando Session Establishment Request.\n✅ [SMF] Sesión Establecida OK (SEID: %d).", ueIP, nuevoSeid)
	responderExito(w, mensajeFront, nuevoSeid)
}

func handleModify(w http.ResponseWriter, r *http.Request) {
	idParam := r.URL.Query().Get("id")
	farParam := r.URL.Query().Get("far")
	mbrParam := r.URL.Query().Get("mbr")

	seid, _ := strconv.ParseUint(idParam, 10, 64)
	
	// Configurar FAR (Forwarding Action Rule). Por defecto, si no se especifica, se mantiene FORWARD.
	var action uint8 = 0x02
	accionStr := "FORWARD (Permitir)"
	if farParam == "drop" { 
		action = 0x01 
		accionStr = "DROP (Bloquear)"
	} else if farParam == "buff" { 
		action = 0x04 
		accionStr = "BUFFER (Retener)"
	}

    
    // Configurar QER (QoS Enforcement Rule - Ancho de Banda)
    var mbr uint64 = 10000000 // 10 Mbps por defecto
    if mbrParam != "" {
        mbr, _ = strconv.ParseUint(mbrParam, 10, 64)
    }

    fmt.Printf("\n[SMF] >>> Ejecutando: MODIFY SEID %d | FAR: %s | MBR: %d bps...\n", seid, accionStr, mbr)

    // Construcción manual del payload MBR para cumplir con TS 29.244 (10 octetos totales)
    mbrBytes := make([]byte, 10)

    // Definimos 5 bytes para Uplink (bytes 0-4) y 5 para Downlink (bytes 5-9)
    binary.BigEndian.PutUint32(mbrBytes[1:5], uint32(mbr))  
    binary.BigEndian.PutUint32(mbrBytes[6:10], uint32(mbr)) 
    
    // Creación del mensaje de modificación
    mod := message.NewSessionModificationRequest(
        0, 0, seid, uint32(2), 0,
        ie.NewUpdateFAR(ie.NewFARID(1), ie.NewApplyAction(action)), 
        ie.NewCreateQER(
            ie.NewQERID(1), 
            ie.NewGateStatus(ie.GateStatusOpen, ie.GateStatusOpen), 
            ie.New(26, mbrBytes), 
        ),
    )


	if err := enviarMensajePFCP(mod, "127.0.0.1"); err != nil {
		responderError(w, "Error de red al modificar las reglas.")
		return
	}
	
	mensajeFront := fmt.Sprintf("\n--- [INYECCIÓN SDN: MODIFICACIÓN] ---\n🎯 Objetivo: SEID %d\n⚙️ Regla FAR: %s\n🚀 Límite QoS (MBR): %d bps\n📤 [SMF] Request enviado al UPF.", seid, accionStr, mbr)
	responderExito(w, mensajeFront, seid)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	idParam := r.URL.Query().Get("id")
	seid, _ := strconv.ParseUint(idParam, 10, 64)
	
	del := message.NewSessionDeletionRequest(0, 0, seid, uint32(3), 0)
	enviarMensajePFCP(del, "127.0.0.1")
	
	mensajeFront := fmt.Sprintf("\n--- [LIBERACIÓN DE RECURSOS] ---\nℹ️ Evento: Borrado de contexto en plano de usuario.\n📤 [SMF] Enviando Deletion Request (SEID: %d).", seid)
	responderExito(w, mensajeFront, seid)
}

func enviarMensajePFCP(msg message.Message, targetIP string) error {
	conn, err := net.DialTimeout("udp", targetIP+":8805", 2*time.Second)
	if err != nil { return err }
	defer conn.Close()

	m, ok := msg.(Marshaler)
	if !ok { return fmt.Errorf("error de serialización") }
	b, _ := m.Marshal()
	
	_, err = conn.Write(b)
	if err != nil { return err }

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	bufferRespuesta := make([]byte, 1024)
	_, err = conn.Read(bufferRespuesta)
	if err != nil {
		if netErr, isNetErr := err.(net.Error); isNetErr && netErr.Timeout() {
			return nil
		}
		return fmt.Errorf("error leyendo respuesta")
	}
	return nil
}

func responderExito(w http.ResponseWriter, msg string, seid uint64) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": msg, "status": "success", "seid": seid})
}

func responderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"message": msg, "status": "error"})
}

func handleUpfLog(w http.ResponseWriter, r *http.Request) {
	var m LogMessage
	json.NewDecoder(r.Body).Decode(&m)
	upfLogsMu.Lock()
	upfLogs = append(upfLogs, m.Message)
	upfLogsMu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func getLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	upfLogsMu.Lock()
	json.NewEncoder(w).Encode(upfLogs)
	upfLogsMu.Unlock()
}