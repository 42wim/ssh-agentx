package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/buptczq/WinCryptSSHAgent/app"
	"golang.org/x/crypto/ssh/agent"
)

var applications = []app.Application{
	new(app.WSL),
	new(app.Cygwin),
	new(app.NamedPipe),
	new(app.Pageant),
	new(app.VSock),
}

func (s *SSHAgent) SSHAgentHandler(conn io.ReadWriteCloser) {
	defer conn.Close()

	if s.ExtendedAgent == nil {
		return
	}

	agent.ServeAgent(s, conn)
}

func (s *SSHAgent) start() {
	ctx := context.Background()
	wg := new(sync.WaitGroup)
	ctx = context.WithValue(ctx, "hv", false)

	for _, app := range applications {
		s.runApplication(ctx, app, wg)
	}

	wg.Wait()
}

func (s *SSHAgent) runApplication(ctx context.Context, myApp app.Application, wg *sync.WaitGroup) {
	cwd, _ := os.Getwd()

	fmt.Println("started", myApp.AppId().String())

	switch myApp.AppId().String() {
	case "Cygwin":
		fmt.Println("set SSH_AUTH_SOCK=" + filepath.Join(cwd, app.CYGWIN_SOCK))
	case "WinSSH":
		fmt.Println("set SSH_AUTH_SOCK=" + app.NAMED_PIPE)
	case "WSL":
		fmt.Println("export SSH_AUTH_SOCK=" + filepath.Join(cwd, app.WSL_SOCK))
	}

	wg.Add(1)

	go func(application app.Application) {
		if application.AppId().String() == "Hyper-V" {
			fmt.Println("WSL2 settings:")
			fmt.Println("   socat UNIX-LISTEN:/tmp/" + agentName + ".sock,fork,mode=777 SOCKET-CONNECT:40:0:x0000x33332222x02000000x00000000,forever,interval=5 &")
			fmt.Println("   export SSH_AUTH_SOCK=/tmp/" + agentName + ".sock")
		}

		err := application.Run(ctx, s.SSHAgentHandler)
		if err != nil {
			if application.AppId().String() == "Hyper-V" {
				fmt.Printf("ERROR: wsl2: %s", err)
				fmt.Println("you can ignore the wsl2 error if you don't have/use wsl2")

				return
			}
		}

		wg.Done()
	}(myApp)
}
