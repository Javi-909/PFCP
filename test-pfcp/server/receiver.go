package main
					//ESTE CODIGO ES PARA HEARTBEAT RESPONSE (solo)
import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func main() {
	addr := ":8805" // escucha en todas las interfaces, puerto 8805
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatalf("Error al abrir puerto UDP: %v", err)
	}
	defer pc.Close()
	fmt.Println("Servidor PFCP escuchando en", addr)

	buf := make([]byte, 4096)

	for {
		n, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			log.Printf("Error leyendo UDP: %v", err)
			continue
		}

		fmt.Printf("\nPaquete recibido de %v (%d bytes)\n", clientAddr, n)

		// Parsear el mensaje PFCP recibido
		msg, err := message.Parse(buf[:n])
		if err != nil {
			log.Printf("Error parseando mensaje PFCP: %v", err)
			continue
		}

		fmt.Printf("Mensaje PFCP recibido:\n%+v\n", msg)

		// Verificar tipo de mensaje: si es Heartbeat Request (1), responder
		if msg.MessageType() == message.MsgTypeHeartbeatRequest {
			fmt.Println("→ Recibido Heartbeat Request, enviando Heartbeat Response...")

			// Crear IE con timestamp de recuperación
			recovery := ie.NewRecoveryTimeStamp(time.Now())

			// Crear respuesta PFCP Heartbeat Response
			resp := message.NewHeartbeatResponse(msg.Sequence(), recovery)

			// Serializar a bytes
			bin, err := resp.Marshal()
			if err != nil {
				log.Printf("Error serializando respuesta: %v", err)
				continue
			}

			// Enviar la respuesta al cliente
			_, err = pc.WriteTo(bin, clientAddr)
			if err != nil {
				log.Printf("Error enviando respuesta: %v", err)
				continue
			}

			fmt.Printf("← Heartbeat Response enviado a %v\n", clientAddr)
		}
	}
}
