package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

var reportAddr = "http://127.0.0.1:8080/api/upf-log"

type Association struct {
	NodeID    string
	Timestamp time.Time
}

type Session struct {
	FSEID  uint64
	NodeID string
	Active bool
	PDRs   map[uint16]*ie.IE
	FARs   map[uint32]*ie.IE
}

var associations = make(map[string]Association)
var sessions = make(map[uint64]Session)

func main() {
	addr := "127.0.0.1:8805"
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatalf("Error al abrir puerto UDP: %v", err)
	}
	defer pc.Close()
	
	fmt.Println("\n=== [UPF] INICIANDO PLANO DE USUARIO (Versión Compatible) ===")

	buf := make([]byte, 4096)
	for {
		n, clientAddr, err := pc.ReadFrom(buf)
		if err != nil { continue }
		msg, err := message.Parse(buf[:n])
		if err != nil {
			continue 
		}
		handleMessage(pc, clientAddr, msg)
	}
}

func logWeb(text string) {
	fmt.Println(text)
	payload := map[string]string{"message": text}
	jsonValue, _ := json.Marshal(payload)
	http.Post(reportAddr, "application/json", bytes.NewBuffer(jsonValue))
}

func handleMessage(pc net.PacketConn, clientAddr net.Addr, msg message.Message) {
	switch msg.MessageType() {

	case message.MsgTypeAssociationSetupRequest:
		logWeb("📥 [RX] Association Setup Request")
		reqAssoc, ok := msg.(*message.AssociationSetupRequest)
		if !ok || reqAssoc.NodeID == nil { return }

		nodeID, _ := reqAssoc.NodeID.NodeID()
		associations[nodeID] = Association{NodeID: nodeID, Timestamp: time.Now()}
		
		logWeb(fmt.Sprintf("   ℹ️  [LÓGICA] Nuevo Nodo de Control detectado (%s).", nodeID))

		// Constructor oficial (Flag = 0)
		resp := message.NewAssociationSetupResponse(
			msg.Sequence(),
			ie.NewCause(ie.CauseRequestAccepted),
			ie.NewRecoveryTimeStamp(time.Now()),
		)
		sendResponse(pc, clientAddr, resp)
		logWeb("   📤 [TX] Association Setup Response (Accepted)")

	case message.MsgTypeSessionEstablishmentRequest:
		logWeb("📥 [RX] Session Establishment Request")
		if len(associations) == 0 { return }
		reqSE, ok := msg.(*message.SessionEstablishmentRequest)
		if !ok { return }

		var cpFSEID *ie.FSEIDFields
		if reqSE.CPFSEID != nil { cpFSEID, _ = reqSE.CPFSEID.FSEID() }
		if cpFSEID == nil { return }

		newSession := Session{
			FSEID: cpFSEID.SEID, NodeID: "127.0.0.1", Active: true,
			PDRs: make(map[uint16]*ie.IE), FARs: make(map[uint32]*ie.IE),
		}

		logWeb(fmt.Sprintf("   ℹ️  [LÓGICA] Creando contexto para F-SEID: %d", cpFSEID.SEID))
		sessions[cpFSEID.SEID] = newSession

		// Constructor oficial
		resp := message.NewSessionEstablishmentResponse(
			0, 0, cpFSEID.SEID, msg.Sequence(), 0,
			ie.NewCause(ie.CauseRequestAccepted),
			ie.NewFSEID(cpFSEID.SEID, net.ParseIP("127.0.0.1"), nil, nil),
		)
		sendResponse(pc, clientAddr, resp)
		logWeb("   📤 [TX] Session Establishment Response (Accepted)")

	case message.MsgTypeSessionModificationRequest:
		logWeb("📥 [RX] Session Modification Request")
		reqMod, _ := msg.(*message.SessionModificationRequest)
		seid := reqMod.SEID()

		session, exists := sessions[seid]
		if !exists {
			logWeb(fmt.Sprintf("   ❌ [ALERTA] Sesión %d no encontrada. Rechazando.", seid))
			
			resp := message.NewSessionModificationResponse(
				0, 0, seid, msg.Sequence(), 0,
				ie.NewCause(ie.CauseSessionContextNotFound),
			)
			sendResponse(pc, clientAddr, resp)
			logWeb("   📤 [TX] Session Modification Response (Rejected)")
			return
		}

		logWeb(fmt.Sprintf("   ℹ️  [LÓGICA] Sesión %d encontrada. Aplicando cambios.", seid))
		
		sessions[seid] = session

		resp := message.NewSessionModificationResponse(
			0, 0, seid, msg.Sequence(), 0,
			ie.NewCause(ie.CauseRequestAccepted),
		)
		sendResponse(pc, clientAddr, resp)
		logWeb("   📤 [TX] Session Modification Response (Accepted)")
	}
}

// Interfaz local para forzar el Marshal
type Marshaler interface {
	Marshal() ([]byte, error)
}

func sendResponse(pc net.PacketConn, addr net.Addr, msg message.Message) {
	// Truco: Convertimos la interfaz genérica a una que sepamos que tiene Marshal
	if m, ok := msg.(Marshaler); ok {
		bin, _ := m.Marshal()
		pc.WriteTo(bin, addr)
	} else {
		fmt.Println("Error: El mensaje no soporta Marshal")
	}
}