package main

// import (
// 	"errors"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net"
// 	"sync"
// )

// const (
// 	proxyPort  = "30120"
// 	targetHost = "212.80.214.124:30120"
// 	bufferSize = 1 << 16 // 64KB buffer size for UDP
// )

// func main() {
// 	// Start UDP proxy in a goroutine
// 	go startUDPProxy(":"+proxyPort, targetHost)

// 	// Start TCP proxy in the main thread
// 	startTCPProxy(":"+proxyPort, targetHost)
// }

// func startUDPProxy(listenAddr, targetAddr string) {
// 	sourceAddr, err := net.ResolveUDPAddr("udp", listenAddr)
// 	if err != nil {
// 		log.Fatalf("Failed to resolve UDP source address: %v", err)
// 	}

// 	targetUDPAddr, err := net.ResolveUDPAddr("udp", targetAddr)
// 	if err != nil {
// 		log.Fatalf("Failed to resolve UDP target address: %v", err)
// 	}

// 	conn, err := net.ListenUDP("udp", sourceAddr)
// 	if err != nil {
// 		log.Fatalf("Failed to listen on UDP: %v", err)
// 	}
// 	defer conn.Close()

// 	log.Printf("UDP proxy listening on %s, forwarding to %s", listenAddr, targetAddr)

// 	// Create a map to track client connections
// 	clientMap := make(map[string]*net.UDPConn)
// 	var clientMapMutex sync.Mutex

// 	for {
// 		buffer := make([]byte, bufferSize)
// 		n, clientAddr, err := conn.ReadFromUDP(buffer)
// 		if err != nil {
// 			log.Printf("Error reading from UDP: %v", err)
// 			continue
// 		}

// 		clientAddrStr := clientAddr.String()

// 		// Get or create connection to target for this client
// 		clientMapMutex.Lock()
// 		targetConn, exists := clientMap[clientAddrStr]
// 		if !exists {
// 			targetConn, err = net.DialUDP("udp", nil, targetUDPAddr)
// 			if err != nil {
// 				log.Printf("Failed to connect to target address for client %s: %v", clientAddrStr, err)
// 				clientMapMutex.Unlock()
// 				continue
// 			}
// 			clientMap[clientAddrStr] = targetConn

// 			// Start a goroutine to handle responses from the target
// 			go handleUDPResponses(targetConn, conn, clientAddr, clientAddrStr, &clientMapMutex, clientMap)
// 		}
// 		clientMapMutex.Unlock()

// 		// Forward client data to target
// 		if _, err := targetConn.Write(buffer[:n]); err != nil {
// 			log.Printf("Error forwarding UDP data to target for client %s: %v", clientAddrStr, err)
// 		}
// 	}
// }

// func handleUDPResponses(targetConn *net.UDPConn, clientListener *net.UDPConn, clientAddr *net.UDPAddr, clientAddrStr string, clientMapMutex *sync.Mutex, clientMap map[string]*net.UDPConn) {
// 	buffer := make([]byte, bufferSize)
// 	for {
// 		n, _, err := targetConn.ReadFromUDP(buffer)
// 		if err != nil {
// 			log.Printf("Error reading UDP response from target for client %s: %v", clientAddrStr, err)

// 			// Clean up the connection
// 			clientMapMutex.Lock()
// 			delete(clientMap, clientAddrStr)
// 			clientMapMutex.Unlock()
// 			targetConn.Close()
// 			return
// 		}

// 		// Forward target's response back to the client
// 		if _, err := clientListener.WriteToUDP(buffer[:n], clientAddr); err != nil {
// 			log.Printf("Error forwarding UDP response to client %s: %v", clientAddrStr, err)
// 		}

// 		// Generate a hex dump of the UDP data received
// 		if n > 0 {
// 			// Print a Wireshark-style hex dump
// 			log.Printf("UDP Response from target to client %s (%d bytes):", clientAddrStr, n)
// 			for i := 0; i < n; i += 16 {
// 				end := i + 16
// 				if end > n {
// 					end = n
// 				}

// 				// Hex values part
// 				hexLine := ""
// 				for j := i; j < end; j++ {
// 					if j%8 == 0 && j != i {
// 						hexLine += " "
// 					}
// 					hexLine += fmt.Sprintf("%02x ", buffer[j])
// 				}

// 				// Padding for alignment if line is short
// 				for j := end; j < i+16; j++ {
// 					if j%8 == 0 && j != i {
// 						hexLine += " "
// 					}
// 					hexLine += "   "
// 				}

// 				// ASCII representation
// 				asciiLine := "  "
// 				for j := i; j < end; j++ {
// 					b := buffer[j]
// 					if b >= 32 && b <= 126 { // Printable ASCII
// 						asciiLine += string(b)
// 					} else {
// 						asciiLine += "."
// 					}
// 				}

// 				log.Printf("%04x  %s%s", i, hexLine, asciiLine)
// 			}
// 		}
// 	}
// }

// func startTCPProxy(listenAddr, targetAddr string) {
// 	listener, err := net.Listen("tcp", listenAddr)
// 	if err != nil {
// 		log.Fatalf("Failed to start TCP listener: %v", err)
// 	}
// 	defer listener.Close()

// 	log.Printf("TCP proxy listening on %s, forwarding to %s", listenAddr, targetAddr)

// 	for {
// 		clientConn, err := listener.Accept()
// 		if err != nil {
// 			log.Printf("Failed to accept TCP connection: %v", err)
// 			continue
// 		}

// 		go handleTCPConnection(clientConn, targetAddr)
// 	}
// }

// func handleTCPConnection(clientConn net.Conn, targetAddr string) {
// 	defer clientConn.Close()

// 	// Connect to target server
// 	targetConn, err := net.Dial("tcp", targetAddr)
// 	if err != nil {
// 		log.Printf("Failed to connect to TCP target: %v", err)
// 		return
// 	}
// 	defer targetConn.Close()

// 	// Setup bidirectional copy
// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	// Copy client -> server
// 	go func() {
// 		defer wg.Done()
// 		_, err := io.Copy(targetConn, clientConn)
// 		switch {
// 		case err == nil:
// 		case errors.Is(err, io.EOF):
// 			// log.Println("Client connection closed")
// 		case errors.Is(err, net.ErrClosed):
// 			// log.Println("Target connection closed")
// 		default:
// 			log.Printf("Error copying TCP data from client to target: %v", err)
// 		}
// 		targetConn.Close()
// 	}()

// 	// Copy server -> client
// 	go func() {
// 		defer wg.Done()
// 		_, err := io.Copy(clientConn, targetConn)
// 		switch {
// 		case err == nil:
// 		case errors.Is(err, io.EOF):
// 			// log.Println("Target connection closed")
// 		case errors.Is(err, net.ErrClosed):
// 			// log.Println("Client connection closed")
// 		default:
// 			log.Printf("Error copying TCP data from target to client: %v", err)
// 		}
// 		clientConn.Close()
// 	}()

// 	// Wait for both goroutines to finish
// 	wg.Wait()
// }
