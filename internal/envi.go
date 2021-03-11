package internal

import (
	kms "cloud.google.com/go/kms/apiv1"
	"context"
	"errors"
	"fmt"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gopkg.in/yaml.v3"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"sort"
)

const (
	Production Environment = "production"
	Developer  Environment = "developer"
	Filename               = "envi.yaml"
	maxDepth               = 100
)

var (
	resourceID               = os.Getenv("ENVI_RESOURCE_ID")
	fs            fileSystem = osFS{stat: os.Stat}
	errResourceID            = errors.New("gcp keystore ENVI_RESOURCE_ID is not set or invalid")
)

type fileSystem interface {
	Stat(name string) (os.FileInfo, error)
}

type osFS struct {
	stat func(string) (os.FileInfo, error)
}

func (fs osFS) Stat(name string) (os.FileInfo, error) {
	return fs.stat(name)
}

type Environment string

func (env Environment) ToString() string {
	return string(env)
}

type EnvVariable struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Env struct {
	Production []*EnvVariable `yaml:"production"`
	Developer  []*EnvVariable `yaml:"developer"`
}

type Envi struct {
	*Env `yaml:"env"`
}

func NewEnvi() *Envi {
	return &Envi{
		&Env{
			Production: []*EnvVariable{
				{
					Name:  "",
					Value: "",
				},
			},
			Developer: []*EnvVariable{
				{
					Name:  "",
					Value: "",
				},
			},
		},
	}
}

func (envi *Envi) Load() (f *os.File, err error) {
	if len(resourceID) == 0 {
		return nil, errResourceID
	}
	enviFile := Filename
	_, b, _, _ := runtime.Caller(1)
	envDir := path.Join(path.Dir(b))

	for i := 0; i < maxDepth; i++ {
		_, err := fs.Stat(path.Join(envDir, Filename))
		if err != nil {
			envDir = path.Join(envDir, "..")
		} else {
			enviFile = path.Join(envDir, Filename)
			break
		}
	}

	cipherText, f, err := readFile(enviFile)
	if err != nil {
		return
	}

	data, err := decryptSymmetric(resourceID, cipherText)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(data, envi)
	return
}

func (envi *Envi) Save(f *os.File) error {
	envi.sortVars(Production)
	envi.sortVars(Developer)
	newData, err := envi.Marshal()
	if err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.WriteAt(newData, 0); err != nil {
		return err
	}

	_ = f.Sync()

	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

func (envi *Envi) ExportVars(environment Environment) error {
	if envi != nil {
		_, err := exportEnviEnvironment(environment, envi)
		return err
	}
	return nil
}

func (envi *Envi) sortVars(environment Environment) {
	switch environment {
	case Production:
		envi.Env.Production = sortVars(envi.Env.Production)
	case Developer:
		envi.Env.Developer = sortVars(envi.Env.Developer)
	}
}

func (envi *Envi) Marshal() ([]byte, error) {
	if len(resourceID) == 0 {
		return nil, errResourceID
	}
	data, err := yaml.Marshal(envi)
	if err != nil {
		return nil, err
	}
	return encryptSymmetric(resourceID, string(data))
}

func (envi *Envi) envVars(environment Environment) []*EnvVariable {
	switch environment {
	case Production:
		return envi.Env.Production
	case Developer:
		return envi.Env.Developer
	}
	return nil
}

func readFile(filename string) ([]byte, *os.File, error) {
	_, err := os.Stat(filename)
	if err != nil {
		return nil, nil, err
	}

	f, err := os.OpenFile(filename, os.O_RDWR, os.ModeExclusive)
	if err != nil {
		return nil, nil, err
	}

	cipherText, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, f, err
	}

	return cipherText, f, nil
}

func exportEnviEnvironment(env Environment, envi *Envi) ([]*EnvVariable, error) {
	var envVariables []*EnvVariable
	switch env {
	case Production:
		envVariables = envi.Env.Production
	case Developer:
		envVariables = envi.Env.Developer
	}
	if err := exportEnviVariables(envVariables); err != nil {
		return envVariables, err
	}
	return envVariables, nil
}

func exportEnviVariables(vars []*EnvVariable) error {
	for _, v := range vars {
		if len(os.Getenv(v.Name)) > 0 {
			continue
		}
		if err := os.Setenv(v.Name, v.Value); err != nil {
			return err
		}
	}
	return nil
}

// decryptSymmetric will decrypt the input ciphertext bytes using the specified symmetric key.
func decryptSymmetric(name string, ciphertext []byte) ([]byte, error) {
	ctx := context.Background()
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create kms client: %v", err)
	}

	crc32c := func(data []byte) uint32 {
		t := crc32.MakeTable(crc32.Castagnoli)
		return crc32.Checksum(data, t)
	}
	ciphertextCRC32C := crc32c(ciphertext)

	req := &kmspb.DecryptRequest{
		Name:             name,
		Ciphertext:       ciphertext,
		CiphertextCrc32C: wrapperspb.Int64(int64(ciphertextCRC32C)),
	}

	result, err := client.Decrypt(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt ciphertext: %v", err)
	}

	if int64(crc32c(result.Plaintext)) != result.PlaintextCrc32C.Value {
		return nil, fmt.Errorf("decrypt: response corrupted in-transit")
	}

	return result.Plaintext, nil
}

func encryptSymmetric(name string, message string) ([]byte, error) {
	ctx := context.Background()
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create kms client: %v", err)
	}

	plaintext := []byte(message)

	crc32c := func(data []byte) uint32 {
		t := crc32.MakeTable(crc32.Castagnoli)
		return crc32.Checksum(data, t)
	}
	plaintextCRC32C := crc32c(plaintext)

	req := &kmspb.EncryptRequest{
		Name:            name,
		Plaintext:       plaintext,
		PlaintextCrc32C: wrapperspb.Int64(int64(plaintextCRC32C)),
	}

	result, err := client.Encrypt(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %v", err)
	}

	if result.VerifiedPlaintextCrc32C == false {
		return nil, fmt.Errorf("encrypt: request corrupted in-transit")
	}
	if int64(crc32c(result.Ciphertext)) != result.CiphertextCrc32C.Value {
		return nil, fmt.Errorf("encrypt: response corrupted in-transit")
	}

	return result.Ciphertext, nil
}

func sortVars(e []*EnvVariable) []*EnvVariable {
	sort.Slice(e, func(i, j int) bool {
		return e[i].Name < e[j].Name
	})
	return e
}
