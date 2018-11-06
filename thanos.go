package thanos

import (
	"log"
	"net"
	"sync/atomic"
	"time"

	"gitlab.meitu.com/platform/thanos/command"
	"gitlab.meitu.com/platform/thanos/context"
)

type Server struct {
	servCtx *context.Server
	lis     net.Listener
	idgen   func() int64
}

func idGenerator(base int64) func() int64 {
	id := base
	return func() int64 {
		return atomic.AddInt64(&id, 1)
	}
}

func New(ctx *context.Server) *Server {
	// id generator starts from 1(the first client's id is 2, the same as redis)
	return &Server{servCtx: ctx, idgen: idGenerator(1)}
}

func (s *Server) Serve(lis net.Listener) error {
	for {
		conn, err := lis.Accept()
		if err != nil {
			return err
		}
		log.Println(conn)
		cliCtx := &context.Client{}
		cliCtx.Authenticated = false
		cliCtx.Multi = false
		cliCtx.Done = make(chan struct{})
		cliCtx.Namespace = "default"
		cliCtx.RemoteAddr = conn.RemoteAddr().String()
		cliCtx.DB = s.servCtx.Store.DB(cliCtx.Namespace, 0)
		cliCtx.ID = s.idgen()
		cliCtx.Created = time.Now()
		cliCtx.Updated = cliCtx.Created
		cliCtx.Close = conn.Close
		s.servCtx.Clients.Store(cliCtx.ID, cliCtx)

		cli := client{
			cliCtx: cliCtx,
			server: s,
			exec:   command.NewExecutor(),
		}
		go func(cli *client) {
			if err := cli.serve(conn); err != nil {
				log.Println(err)
			}
			s.servCtx.Clients.Delete(cli.cliCtx.ID)
		}(&cli)
	}
	return nil
}

func (s *Server) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.lis = lis
	return s.Serve(lis)
}

func (s *Server) Stop() error {
	return s.lis.Close()
}

func (s *Server) GracefulStop() error {
	//TODO close client connections gracefully
	return s.lis.Close()
}