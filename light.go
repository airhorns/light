package main

import (
	"io"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"time"
)

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "200 OK")
}

func serve(port string) {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func readAndPrint(reader io.Reader, output chan string) {
		for {
      buf := make([]byte, 1000000)
			_, err := reader.Read(buf)
      if err == io.EOF {
        break
      } else {
        checkErr(err)
      }
      output <- string(buf)
		}
}

func spawnTunnel(id int, port string) (cmd *exec.Cmd, address string) {
	tunnelAddress := regexp.MustCompile(`http://(\w+)\.localtunnel(?:-beta)?\.com`)

	log.Printf("Spawning tunnel #%v.\n", id)
	/*cmd = exec.Command("bundle", "exec", "localtunnel", port)*/
  cmd = exec.Command("script", "/dev/null", "bundle", "exec", "localtunnel", port)

	stdout, err := cmd.StdoutPipe()
	checkErr(err)
	stderr, err := cmd.StderrPipe()
	checkErr(err)
  stdin, err := cmd.StdinPipe()
	checkErr(err)

	errs := make(chan string, 1)
	outs := make(chan string, 1)

	err = cmd.Start()
	checkErr(err)

	go readAndPrint(stderr, errs)
	go readAndPrint(stdout, outs)

  err = stdin.Close()
	checkErr(err)
  log.Printf("cmd: %+v", cmd)

	select {
	case err := <-errs:
		log.Fatal(err)
	case str := <-outs:
		log.Print(str)
		matches := tunnelAddress.FindStringSubmatch(str)
		if matches != nil {
			address = matches[1]
			break
		}
	case <-time.After(1 * time.Minute):
		log.Fatalf("Spawning tunnel %v timed out.", id)
	}
	return
}

func spawnAndRespawnTunnel(id int, port string, addresses chan string) {
	for {
		cmd, address := spawnTunnel(id, port)
		addresses <- address
		tunnelQuit := make(chan error, 1)
		go func() {
			tunnelQuit <- cmd.Wait()
		}()

		select {
		case err := <-tunnelQuit:
			if err != nil {
				log.Print(err)
			}
			break
		case <-time.After(60 * time.Minute):
			break
		}
	}
}

func main() {
	port := flag.String("port", "8080", "the port to run the http server on")
	tunnels := flag.Int("tunnels", 1, "the number of tunnels to run")
	go serve(*port)

	log.Printf("Server started on port %v.\n", *port)

	addresses := make(chan string)
	for i := 0; i < *tunnels; i++ {
		go spawnAndRespawnTunnel(i, *port, addresses)
	}

	for {
		select {
		case address := <-addresses:
			log.Printf("Tunnel ID %v started started on port %v.\n", address, *port)
		}
	}
}
