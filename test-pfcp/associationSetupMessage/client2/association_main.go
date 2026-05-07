package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func main() {
	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8805")
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatalf("Error conectando con servidor PFCP: %v", err)
	}
	defer conn.Close()

	fmt.Println("[SMF] → Enviando Association Setup Request...")

	// Crear IEs necesarias para el mensaje
	nodeID := ie.NewNodeID("127.0.0.1", "", "")
	recovery := ie.NewRecoveryTimeStamp(time.Now())

	// Crear mensaje PFCP Association Setup Request
	msg := message.NewAssociationSetupRequest(1, nodeID, recovery)

	// Serializar
	bin, err := msg.Marshal()
	if err != nil {
		log.Fatalf("Error serializando mensaje PFCP: %v", err)
	}

	// Enviar al UPF
	_, err = conn.Write(bin)
	if err != nil {
		log.Fatalf("Error enviando mensaje PFCP: %v", err)
	}

	// Esperar respuesta
	buffer := make([]byte, 1500)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := conn.ReadFrom(buffer)
	if err != nil {
		log.Println("[SMF] ⚠️ No se recibió respuesta (timeout)")
		return
	}

	resp, err := message.Parse(buffer[:n])
	if err != nil {
		log.Println("Error parseando respuesta:", err)
		return
	}

	fmt.Printf("[SMF] ← Respuesta recibida: %v\n", resp.MessageTypeName())
}
