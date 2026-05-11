package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

// URL del SMF_WEB
var reportAddr = "http://127.0.0.1:8080/api/upf-log"

// Association representa la relación a nivel de nodo
type Association struct {
	NodeID    string
	Timestamp time.Time
}

// Session representa el contexto de un usuario individual
type Session struct {
	FSEID  uint64
	NodeID string
	Active bool
	PDRs   map[uint16]*ie.IE
	FARs   map[uint32]*ie.IE
}

// Utilizamos RWMutex para permitir múltiples lecturas simultáneas pero solo una escritura
var (
	associationsMu sync.RWMutex
	associations   = make(map[string]Association)

	sessionsMu sync.RWMutex
	sessions   = make(map[uint64]Session)
)

func main() {
	addr := "127.0.0.1:8805"
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatalf("Error al abrir puerto UDP: %v", err)
	}
	defer pc.Close()

	fmt.Println("\n=== [UPF MULTI-SESIÓN] PLANO DE USUARIO CONCURRENTE (v4) ===")
	fmt.Printf("Escuchando peticiones PFCP en %s...\n", addr)

	buf := make([]byte, 4096)
	for {
		n, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}

		msgData := make([]byte, n)
		copy(msgData, buf[:n])

		// Lanzamos una Goroutine por cada mensaje recibido.
		// Permite gestionar múltiples sesiones de forma simultánea.
		go func(data []byte, remoteAddr net.Addr) {
			msg, err := message.Parse(data)
			if err != nil {
				return
			}
			handleMessage(pc, remoteAddr, msg)
		}(msgData, clientAddr)
	}
}

// logWeb envía trazas a la interfaz gráfica sin bloquear el procesado de paquetes.
func logWeb(text string) {
	fmt.Println(text)
	payload := map[string]string{"message": text}
	jsonValue, _ := json.Marshal(payload)

	go func() {
		http.Post(reportAddr, "application/json", bytes.NewBuffer(jsonValue))
	}()
}

func handleMessage(pc net.PacketConn, clientAddr net.Addr, msg message.Message) {
	switch msg.MessageType() {

	case message.MsgTypeAssociationSetupRequest:
		logWeb("📥 [RX] Association Setup Request")
		reqAssoc, ok := msg.(*message.AssociationSetupRequest)
		if !ok || reqAssoc.NodeID == nil {
			return
		}

		nodeID, _ := reqAssoc.NodeID.NodeID()

		// Escritura segura: Bloqueamos acceso exclusivo mientras guardamos el nodo.
		associationsMu.Lock()
		associations[nodeID] = Association{NodeID: nodeID, Timestamp: time.Now()}
		associationsMu.Unlock()

		logWeb(fmt.Sprintf("  ℹ️  [LÓGICA] Nuevo Nodo detectado (%s). Guardado en estado.", nodeID))

		resp := message.NewAssociationSetupResponse(
			msg.Sequence(),
			ie.NewCause(ie.CauseRequestAccepted),
			ie.NewRecoveryTimeStamp(time.Now()),
		)
		sendResponse(pc, clientAddr, resp)
		logWeb("  📤 [TX] Association Response (Accepted)")

	case message.MsgTypeSessionEstablishmentRequest:
		logWeb("📥 [RX] Session Establishment Request")
		
		// Lectura segura: Comprobamos si hay asociación sin bloquear otros lectores.
		associationsMu.RLock()
		numAssoc := len(associations)
		associationsMu.RUnlock()

		if numAssoc == 0 {
			logWeb("  ⚠️  [RECHAZO] No se puede crear sesión sin asociación previa.")
			return
		}

		reqSE, ok := msg.(*message.SessionEstablishmentRequest)
		if !ok || reqSE.CPFSEID == nil {
			return
		}

		cpFSEID, _ := reqSE.CPFSEID.FSEID()

		sessionsMu.Lock()
		sessions[cpFSEID.SEID] = Session{
			FSEID: cpFSEID.SEID, NodeID: "127.0.0.1", Active: true,
			PDRs: make(map[uint16]*ie.IE), FARs: make(map[uint32]*ie.IE),
		}
		currentActive := len(sessions)
		sessionsMu.Unlock()

		logWeb(fmt.Sprintf("  ℹ️  [LÓGICA] Creando contexto de túnel para F-SEID: %d. Usuarios en paralelo: %d", cpFSEID.SEID, currentActive))
		logWeb("  ⚙️  [DATOS] Instalando PDR 1: Filtro de paquetes activado.")
		logWeb("  ⚙️  [DATOS] Instalando FAR 1: Regla de reenvío configurada.")

		resp := message.NewSessionEstablishmentResponse(
			0, 0, cpFSEID.SEID, msg.Sequence(), 0,
			ie.NewCause(ie.CauseRequestAccepted),
			ie.NewFSEID(cpFSEID.SEID, net.ParseIP("127.0.0.1"), nil, nil),
		)
		sendResponse(pc, clientAddr, resp)
		logWeb("  📤 [TX] Session Response (Accepted)")

	case message.MsgTypeSessionModificationRequest:
		reqMod, _ := msg.(*message.SessionModificationRequest)
		seid := reqMod.SEID()
		logWeb(fmt.Sprintf("📥 [RX] Session Modification para SEID: %d", seid))

		sessionsMu.RLock()
		session, exists := sessions[seid]
		sessionsMu.RUnlock()

		if !exists {
			logWeb(fmt.Sprintf("  ❌ [ERROR] SEID %d no existe en memoria. Rechazando.", seid))
			resp := message.NewSessionModificationResponse(0, 0, seid, msg.Sequence(), 0, ie.NewCause(ie.CauseSessionContextNotFound))
			sendResponse(pc, clientAddr, resp)
			return
		}

		logWeb(fmt.Sprintf("  ℹ️  [LÓGICA] Sesión %d localizada. Actualizando reglas...", seid))
		logWeb("  🔄 [DATOS] Actualizando PDR 1: QoS modificada.")
		logWeb("  ➕ [DATOS] Añadiendo FAR 2: Nueva ruta creada.")
		
		// Modificamos el estado de forma segura
		sessionsMu.Lock()
		session.Active = true
		sessions[seid] = session
		sessionsMu.Unlock()

		resp := message.NewSessionModificationResponse(0, 0, seid, msg.Sequence(), 0, ie.NewCause(ie.CauseRequestAccepted))
		sendResponse(pc, clientAddr, resp)
		logWeb("  📤 [TX] Session Modification (Accepted)")
	

	case message.MsgTypeSessionDeletionRequest:
        reqDel, _ := msg.(*message.SessionDeletionRequest)
        seid := reqDel.SEID()
        logWeb(fmt.Sprintf("📥 [RX] Session Deletion para SEID: %d", seid))

        // Eliminación física del mapa de sesiones.
        sessionsMu.Lock()
        _, exists := sessions[seid]
        if exists {
            delete(sessions, seid)
        }
        currentActive := len(sessions)
        sessionsMu.Unlock()

        if !exists {
            logWeb(fmt.Sprintf("  ⚠️  [LOG] SEID %d no encontrado al intentar borrar.", seid))
        } else {
            logWeb(fmt.Sprintf("  🗑️  [LÓGICA] Contexto SEID %d eliminado de memoria. Usuarios en paralelo: %d", seid, currentActive))
        }

        // Respondemos al SMF confirmando que la sesión ha sido purgada.
        resp := message.NewSessionDeletionResponse(
            0, 0, seid, msg.Sequence(), 0, 
            ie.NewCause(ie.CauseRequestAccepted),
        )
        sendResponse(pc, clientAddr, resp)
        logWeb("  📤 [TX] Session Deletion Response (Accepted)")
	}
}    

type Marshaler interface {
	Marshal() ([]byte, error)
}

func sendResponse(pc net.PacketConn, addr net.Addr, msg message.Message) {
	if m, ok := msg.(Marshaler); ok {
		bin, _ := m.Marshal()
		pc.WriteTo(bin, addr)
	}
}
