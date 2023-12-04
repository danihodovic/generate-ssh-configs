package main

import (
	"context"
	"fmt"
	"html/template"
	"os"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

const tmpl = `
{{ range $_, $d := .Instances -}}
Host {{ $.Prefix }}{{ $d.Name }}
  HostName {{ ( index $d.Networks.V4 1).IPAddress }}
{{ end}}

Host {{ .Prefix }}*
  User {{ .User }}
`

func generateDigitalOcean(prefix string) {
	tokenSource := &TokenSource{}
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	client := godo.NewClient(oauthClient)

	droplets, _, err := client.Droplets.List(context.TODO(), &godo.ListOptions{PerPage: 200})
	checkErr(err)

	templateData := struct {
		Prefix    string
		User      string
		Instances []godo.Droplet
	}{
		Prefix:    prefix,
		User:      sshUser,
		Instances: droplets,
	}

	t := template.Must(template.New("tmpl").Parse(tmpl))
	err = t.Execute(os.Stdout, templateData)
	checkErr(err)
}

type TokenSource struct{}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	envVar := "DIGITAL_OCEAN_TOKEN"

	envToken := os.Getenv(envVar)
	if envToken == "" {
		fmt.Printf("Please provide a personal access token as %s\n", envVar)
		os.Exit(1)
	}

	token := &oauth2.Token{
		AccessToken: envToken,
	}
	return token, nil
}
