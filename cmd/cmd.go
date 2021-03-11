package main

import (
	"errors"
	"fmt"
	"github.com/catmullet/deferror"
	"github.com/catmullet/envi/internal"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
)

var (
	errorAlreadyInitialized = errors.New("envi.yaml is already initialized")
)

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:    "init",
				Aliases: []string{"i"},
				Usage:   "initialize a new envi envi.yaml",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "resource-id",
						Aliases: []string{"r"},
						Usage:   "GCP resource id for keystore and key",
						EnvVars: []string{"ENVI_RESOURCE_ID"},
					},
				},
				Action: initYaml,
			},
			{
				Name:    "edit",
				Aliases: []string{"e"},
				Usage:   "edit an environment file, envi.yaml",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "resource-id",
						Aliases: []string{"r"},
						Usage:   "GCP resource id for keystore and key",
						EnvVars: []string{"ENVI_RESOURCE_ID"},
					},
				},
				Action: edit,
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func initYaml(c *cli.Context) error {
	var newEnvi = internal.NewEnvi()
	_, err := newEnvi.Load()
	if err == nil {
		return errorAlreadyInitialized
	}
	data, err := newEnvi.Marshal()
	if err != nil {
		return err
	}

	f, err := os.Create(internal.Filename)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		fmt.Println(err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func edit(c *cli.Context) (err error) {
	var currentEnvi = internal.NewEnvi()
	var modEnvi = internal.NewEnvi()
	var tmpFilename = filepath.Join(os.TempDir(), internal.Filename+".tmp")

	cf, err := currentEnvi.Load()
	if err != nil {
		return err
	}

	defer deferror.As(cf.Close, &err)

	nf, err := os.Create(tmpFilename)
	if err != nil {
		return err
	}

	currentBytes, err := yaml.Marshal(currentEnvi)
	if err != nil {
		return err
	}

	if _, err := nf.Write(currentBytes); err != nil {
		return err
	}

	_ = nf.Sync()
	if err := nf.Close(); err != nil {
		return err
	}

	if err := startEditor(nf); err != nil {
		return fmt.Errorf("failure to edit envi.yaml file: %w", err)
	}

	nf, err = os.Open(nf.Name())
	if err != nil {
		return err
	}

	defer os.Remove(tmpFilename)
	defer deferror.As(nf.Close, &err)

	data, err := ioutil.ReadAll(nf)
	if err != nil {
		return err
	}

	// see if it unmarshals to verify
	if err := yaml.Unmarshal(data, modEnvi); err != nil {
		return err
	}

	if err := modEnvi.Save(cf); err != nil {
		return err
	}

	return nil
}

func startEditor(f *os.File) error {
	var cmd *exec.Cmd
	var editor = os.Getenv("EDITOR")
	if len(editor) == 0 {
		editor = "vim"
	}
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command(filepath.Join(os.Getenv("SYSTEMROOT"), "System32", "rundll32.exe"),
			"url.dll,FileProtocolHandler", f.Name())
	case "darwin":
		cmd = exec.Command(editor, f.Name())
	case "linux":
		cmd = exec.Command(editor, f.Name())
	default:
		return errors.New("operating system not supported")
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
