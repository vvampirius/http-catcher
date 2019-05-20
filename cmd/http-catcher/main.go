package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	kitlog "github.com/go-kit/kit/log"
	loglevel "github.com/go-kit/kit/log/level"
	githubWebHookLib "github.com/vvampirius/http-catcher/githubWebHook"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)


const VERSION  = 0.2

var (
	Logger = kitlog.NewLogfmtLogger(os.Stdout)
)


// https://stackoverflow.com/questions/39320025/how-to-stop-http-listenandserve
func startHttpServer(addr string) (*http.Server, chan error) {
	server := &http.Server{Addr: addr}
	httpErrorChan := make(chan error, 1)

	go func() {
		err := server.ListenAndServe()

		// fail if not ErrServerClosed on graceful close
		if err != http.ErrServerClosed {
			// NOTE: there is a chance that next line won't have time to run,
			// as main() doesn't wait for this goroutine to stop. don't use
			// code with race conditions like these for production. see post
			// comments below on more discussion on how to handle this.
			loglevel.Error(Logger).Log(`msg`, fmt.Sprintf("ListenAndServe(): %s", err))
			os.Exit(1)
		}

		httpErrorChan <- err
	}()

	return server, httpErrorChan
}


func main() {
	listen := flag.String("l", `:8080`, "addr to listen")
	githubExecurosPath := flag.String("g", `/tmp`, "Path to executables for Github Web Hooks")
	ver := flag.Bool("v", false, "Show version")
	flag.Parse()

	if *ver == true {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	loglevel.Info(Logger).Log(`msg`, fmt.Sprintf("Started version %v", VERSION))

	httpServer, httpErrorChan := startHttpServer(*listen)

	mainContext, mainContextCancel := context.WithCancel(context.Background())

	githubWebHook := githubWebHookLib.NewHook(*githubExecurosPath)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-signalChan
		loglevel.Info(Logger).Log(`msg`, fmt.Sprintf("Got signal: %v", s))
		httpServer.Shutdown(context.TODO())
		loglevel.Info(Logger).Log(`msg`, fmt.Sprintf("HTTP exited with error: '%v'", <-httpErrorChan))
		mainContextCancel()
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//io.WriteString(w, "hello world\n")
		loglevel.Info(Logger).Log(`Host`, r.Host, `Method`, r.Method, `RemoteAddr`, r.RemoteAddr,
			`RequestURI`, r.RequestURI)
		githubWebHook.Handler(r)

		for header, value := range r.Header {
			fmt.Printf("%s: %s\n", header, value)
		}
		if r.Method == `POST` || r.Method == `PUT` {
			if r.Body != nil {
				reader := bufio.NewReader(r.Body)
				for {
					line, err := reader.ReadString('\n')
					fmt.Println(line)
					if err != nil { break }
				}
			}
		}
	})

	<-mainContext.Done()

	loglevel.Info(Logger).Log(`msg`, fmt.Sprintf("Finished version %v", VERSION))
}