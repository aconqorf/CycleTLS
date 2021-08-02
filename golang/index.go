package main

import (
	// "encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	// "time"
	// "io"
	"google.golang.org/grpc"
	"fmt"
	"net"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/examples/data"
	// "github.com/gorilla/websocket"
	pb "github.com/Danny-Dasilva/gRPC-Tests/bidirectional/js-test/cycletlsproto"
)

//rename to request+client+options
type fullRequest struct {
	req     *http.Request
	client  http.Client
	options pb.CycleTLSRequest
}

//TODO: rename this response struct
type respData struct {
	Status  int
	Body    string
	Headers map[string]string
}

//Response contains Cycletls response data
type Response struct {
	RequestID string
	Response  respData
}

//CycleTLS creates full request and response
type CycleTLS struct {
	ReqChan  chan fullRequest
	RespChan chan Response
}

func getWebsocketAddr() string {
	port, exists := os.LookupEnv("WS_PORT")

	var addr *string

	if exists {
		addr = flag.String("addr", "localhost:"+port, "http service address")
	} else {
		addr = flag.String("addr", "localhost:9112", "http service address")
	}
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/"}

	return u.String()
}

// ready Request
func processRequest(request pb.CycleTLSRequest) (result fullRequest) {

	var browser = browser{
		JA3:       request.Ja3,
		UserAgent: request.UserAgent,
		Cookies:   request.Cookies,
	}

	client, err := newClient(browser, request.Proxy)
	if err != nil {
		log.Fatal(err)
	}
	req, err := http.NewRequest(strings.ToUpper(request.Method), request.URL, strings.NewReader(request.Body))
	if err != nil {
		log.Print(request.RequestID + "Request_Id_On_The_Left" + err.Error())
		return
	}
	// for k, v := range request.Headers {
	// 	if k != "host" {
	// 		req.Header.Set(k, v)
	// 	}
	// } TODO fix this
	return fullRequest{req: req, client: client, options: request}

}

func dispatcher(res fullRequest) (response pb.Response, err error) {
	resp, err := res.client.Do(res.req)
	if err != nil {

		parsedError := parseError(err)

		headers := make(map[string]string)
		//TODO fix RequestID
		response := pb.Response{Status: parsedError.StatusCode, Body: parsedError.ErrorMsg + "-> \n" + string(err.Error()), Headers: headers}
		return response, err
		// return response, err

	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Parse Bytes" + err.Error())
		return response, err
	}

	headers := make(map[string]string)

	for name, values := range resp.Header {
		if name == "Set-Cookie" {
			headers[name] = strings.Join(values, "/,/")
		} else {
			for _, value := range values {
				headers[name] = value
			}
		}
	}

	response = pb.Response{Status: int32(resp.StatusCode), Body: string(bodyBytes), Headers: headers}
	return response, nil

}

// Queue queues request in worker pool TODO fix this
// func (client CycleTLS) Queue(URL string, options Options, Method string) {

// 	options.URL = URL
// 	//TODO add timestamp to request
// 	opt := cycleTLSRequest{"n", options}
// 	response := processRequest(opt)
// 	client.ReqChan <- response
// }

// // Do creates a single request //TODO fix this
// func (client CycleTLS) Do(URL string, options Options, Method string) (response Response, err error) {

// 	options.URL = URL

// 	opt := cycleTLSRequest{"n", options}

// 	res := processRequest(opt)
// 	response, err = dispatcher(res)
// 	if err != nil {
// 		log.Print("Request Failed: " + err.Error())
// 		return response, err
// 	}

// 	return response, nil
// }

//TODO rename this

// Init starts the worker pool or returns a empty cycletls struct TODO fix this
// func Init(workers ...bool) CycleTLS {

// 	if len(workers) > 0 && workers[0] {
// 		reqChan := make(chan fullRequest)
// 		respChan := make(chan Response)
// 		go workerPool(reqChan, respChan)
// 		log.Println("Worker Pool Started")

// 		return CycleTLS{ReqChan: reqChan, RespChan: respChan}
// 	}
// 	return CycleTLS{}

// }

// Close closes channels
func (client CycleTLS) Close() {
	close(client.ReqChan)
	close(client.RespChan)

}

// Worker Pool
func workerPool(reqChan chan fullRequest, respChan chan pb.Response) {
	//MAX
	for i := 0; i < 100; i++ {
		go worker(reqChan, respChan)
	}
}

// Worker
func worker(reqChan chan fullRequest, respChan chan pb.Response) {
	for res := range reqChan {
		response, err := dispatcher(res)
		if err != nil {
			log.Print("Request Failed: " + err.Error())
		}
		respChan <- response
	}
}


var (
	reqChan = make(chan fullRequest)
    respChan = make(chan pb.Response)
	usetls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile   = flag.String("cert_file", "", "The TLS cert file")
	keyFile    = flag.String("key_file", "", "The TLS key file")
	port       = flag.Int("port", 10000, "The server port")

)

type routeGuideServer struct {
	pb.UnimplementedCycleStreamServer
}


func (s *routeGuideServer) Stream(stream pb.CycleStream_StreamServer) error {

	go func () {
		for {
			in, _ := stream.Recv()
			
			var req = in 
			// rn := pb.CycleTLSRequest{RequestID: request.RequestID, Options: &pb.Options{URL: request.Options.URL, Method: request.Options.Method, Headers: request.Options.Headers, Body: request.Options.Body, Ja3: request.Options.Ja3, Proxy: request.Options.Proxy, Cookies: request.Options.Cookies}}
			if req != nil {
				log.Println(req.URL)
				rn := pb.CycleTLSRequest{RequestID: req.RequestID, URL: req.URL, Method: req.Method, Headers: req.Headers, Body: req.Body, Ja3: req.Ja3, Proxy: req.Proxy, Cookies: req.Cookies}
				reply := processRequest(rn)
				reqChan <- reply
			}
		}
		

	}()
	for {
        headers := make(map[string]string)
		select {
			case r := <-respChan:
				response := &pb.Response{RequestID: "1", Status: 200, Body: "someshit", Headers: headers}
				_=response
				log.Println("1")
				if err := stream.Send(&r); err != nil {
					log.Println(err)
					return err
				}
				_=r
			default:
			}	
	}
	//run as main thread
	

	// for {
	// 	select {
	// 	case r := <-respChan:
	// 		if err := stream.Send(&r); err != nil {
	// 			log.Println(err)
	// 			return err
	// 		}
	// 	default:
	// 	}

	// }

}

func newServer() *routeGuideServer {
	s := &routeGuideServer{}
	return s
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	// start := time.Now()
	// defer func() {
	// 	log.Println("Execution Time: ", time.Since(start))
	// }()

	
	// flag.Parse()
	
	go workerPool(reqChan, respChan)

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if *usetls {
		if *certFile == "" {
			*certFile = data.Path("x509/server_cert.pem")
		}
		if *keyFile == "" {
			*keyFile = data.Path("x509/server_key.pem")
		}
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterCycleStreamServer(grpcServer, newServer())
	grpcServer.Serve(lis)

}
