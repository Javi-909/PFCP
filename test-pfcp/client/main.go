package main
					//ESTE COGIDO ES PARA HEARTBEAT REQUEST (cliente)
import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func main() {
	// 1) construir IE y mensaje PFCP Heartbeat
	recovery := ie.NewRecoveryTimeStamp(time.Now())
	// En tu versión actual pasabas nil como segundo arg y funcionaba; si no hace falta, se puede omitir
	msg := message.NewHeartbeatRequest(1, recovery, nil)

	// 2) serializar
	bin, err := msg.Marshal()
	if err != nil {
		log.Fatalf("Error al codificar PFCP: %v", err)
	}
	fmt.Printf("Bytes PFCP (hex): %x\n", bin)

	// 3) enviar por UDP a destino PFCP (IP:port)
	dstAddr := "127.0.0.1:8805"

	// DialUDP devuelve una conexión desde un puerto local efímero -> permite leer respuesta
	raddr, err := net.ResolveUDPAddr("udp", dstAddr)
	if err != nil {
		log.Fatalf("ResolveUDPAddr error: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		log.Fatalf("DialUDP error: %v", err)
	}
	defer conn.Close()

	// escribe los bytes
	_, err = conn.Write(bin)
	if err != nil {
		log.Fatalf("Error al enviar UDP: %v", err)
	}
	fmt.Printf("Heartbeat enviado a %s\n", dstAddr)

	// 4) esperar respuesta (opcional) con timeout
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 4096)
	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		// puede caducar si no hay respuesta; lo imprimimos como info, no fatal
		fmt.Printf("No se recibió respuesta en 5s (error/read): %v\n", err)
		return
	}
	fmt.Printf("Respuesta recibida desde %v (%d bytes)\nHex: %x\n", addr, n, buf[:n])
	// parsear posible mensaje PFCP
	parsed, perr := message.Parse(buf[:n])
	if perr != nil {
		fmt.Printf("Error parseando respuesta: %v\n", perr)
	} else {
		fmt.Printf("Mensaje PFCP parseado de respuesta:\n%+v\n", parsed)
	}
}
