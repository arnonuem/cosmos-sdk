package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	rpcserver "github.com/tendermint/tendermint/rpc/lib/server"
	"github.com/tendermint/tmlibs/cli"
	tmlog "github.com/tendermint/tmlibs/log"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/commands"
	rest "github.com/cosmos/cosmos-sdk/client/rest"
	coinrest "github.com/cosmos/cosmos-sdk/modules/coin/rest"
	noncerest "github.com/cosmos/cosmos-sdk/modules/nonce/rest"
	rolerest "github.com/cosmos/cosmos-sdk/modules/roles/rest"
)

var srvCli = &cobra.Command{
	Use:   "baseserver",
	Short: "Light REST client for tendermint",
	Long:  `Baseserver presents  a nice (not raw hex) interface to the basecoin blockchain structure.`,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the light REST client for tendermint",
	Long:  "Access basecoin via REST",
	RunE:  serve,
}

const (
	envPortFlag = "port"
	defaultAlgo = "ed25519"
)

func init() {
	_ = serveCmd.PersistentFlags().Int(envPortFlag, 8998, "the port to run the server on")
}

func serve(cmd *cobra.Command, args []string) error {
	router := mux.NewRouter()

	rootDir := viper.GetString(cli.HomeFlag)
	keyMan := client.GetKeyManager(rootDir)
	serviceKeys := rest.NewServiceKeys(keyMan)

	rpcClient := commands.GetNode()
	serviceTxs := rest.NewServiceTxs(rpcClient)

	routeRegistrars := []func(*mux.Router) error{
		// rest.Keys handlers
		serviceKeys.RegisterCRUD,

		// Coin send handler
		coinrest.RegisterAll,

		// Roles createRole handler
		rolerest.RegisterCreateRole,

		// Basecoin sign transactions handler
		serviceKeys.RegisterSignTx,
		// Basecoin post transaction handler
		serviceTxs.RegisterPostTx,

		// Nonce query handler
		noncerest.RegisterQueryNonce,
	}

	for _, routeRegistrar := range routeRegistrars {
		if err := routeRegistrar(router); err != nil {
			log.Fatal(err)
		}
	}

	port := viper.GetInt(envPortFlag)
	addr := fmt.Sprintf(":%d", port)

	routes := client.RPCRoutes(rpcClient)
	wm := rpcserver.NewWebsocketManager(routes, rpcserver.EventSubscriber(rpcClient))
	wsLogger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout)).With("module", "ws")
	wm.SetLogger(wsLogger)
	router.HandleFunc("/websocket", wm.WebsocketHandler)

	log.Printf("Serving on %q", addr)
	return http.ListenAndServe(addr, router)
}

func main() {
	commands.AddBasicFlags(srvCli)

	srvCli.AddCommand(
		commands.InitCmd,
		commands.VersionCmd,
		serveCmd,
	)

	// this should share the dir with basecli, so you can use the cli and
	// the api interchangeably
	cmd := cli.PrepareMainCmd(srvCli, "BC", os.ExpandEnv("$HOME/.basecli"))
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
