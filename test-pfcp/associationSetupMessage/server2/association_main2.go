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
	addr := ":8805"
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatalf("Error al abrir puerto UDP: %v", err)
	}
	defer pc.Close()
	fmt.Println("[UPF] Esperando mensajes PFCP en", addr)

	buf := make([]byte, 4096)

	for {
		n, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			log.Printf("Error leyendo UDP: %v", err)
			continue
		}

		msg, err := message.Parse(buf[:n])
		if err != nil {
			log.Printf("Error parseando mensaje PFCP: %v", err)
			continue
		}

		fmt.Printf("\n[UPF] Mensaje recibido de %v: %v\n", clientAddr, msg.MessageTypeName())

		switch msg.MessageType() {
		case message.MsgTypeAssociationSetupRequest:
			fmt.Println("[UPF] → Association Setup Request recibido")

			// Crear respuesta
			cause := ie.NewCause(ie.CauseRequestAccepted)
			recovery := ie.NewRecoveryTimeStamp(time.Now())

			resp := message.NewAssociationSetupResponse(msg.Sequence(), cause, recovery)

			bin, err := resp.Marshal()
			if err != nil {
				log.Printf("Error serializando respuesta: %v", err)
				continue
			}

			_, err = pc.WriteTo(bin, clientAddr)
			if err != nil {
				log.Printf("Error enviando respuesta: %v", err)
				continue
			}

			fmt.Println("[UPF] ← Association Setup Response enviado")

		default:
			fmt.Printf("[UPF] ⚠️ Tipo de mensaje PFCP no manejado: %v\n", msg.MessageTypeName())
		}
	}
}
