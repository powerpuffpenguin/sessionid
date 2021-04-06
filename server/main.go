package main

import (
	"os"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/powerpuffpenguin/sessionid/agent"
	"github.com/spf13/cobra"
)

func main() {
	var (
		url string
	)
	root := &cobra.Command{
		Use:   "server",
		Short: "sessionid server",
		Run: func(cmd *cobra.Command, args []string) {
			a, e := newAgent(url)
			if e != nil {
				return
			}
			a.Close()
		},
	}
	flags := root.Flags()
	flags.StringVarP(&url, `url`, `u`,
		`redis`,
		`storage backend [redis://,memory://]`,
	)
	e := root.Execute()
	if e != nil {
		os.Exit(1)
	}
}
func newAgent(url string) (a agent.Agent, e error) {
	if strings.HasPrefix(url, `memory://`) {
		a = agent.NewMemoryAgent()
	} else if strings.HasPrefix(url, `redis://`) {
		var opts *redis.Options
		opts, e = redis.ParseURL(url)
		if e != nil {
			return
		}
		client := redis.NewClient(opts)
		a = agent.NewRedisAgent(client)
	}
	return
}
