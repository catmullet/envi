# envi
Encrypted local environment variable storage

## Getting Started
#### Set your `ENVI_RESOURCE_ID` environment variable
- This will be the symmetric key resource ID stored in GCP's Key Store (https://cloud.google.com/kms/docs/creating-keys). For example `projects/my-project/locations/global/keyRings/my-keyring/cryptoKeys/mykey`

#### Import envi into your project
```go
import github.com/catmullet/envi
```

#### Install the CLI
```shell
go install github.com/catmullet/envi/envi-cli
```

## Running with it
#### Initialize and edit an envi.yaml within your project.
![](https://raw.githubusercontent.com/catmullet/envi/assets/envi_edit.gif)
- From the root of your project run ```envi-cli init```.
- To edit the file run ```envi-cli edit```. This will start up either your default editor defined by the `EDITOR` environment variable or it will default to vim.
#### Run with your `envi.SetEnv(<environment>)` function.
```go
package main

import (
	"fmt"
	"github.com/catmullet/envi"
	"os"
)

func main() {
	if err := envi.SetEnv(envi.Developer); err != nil {
		fmt.Println("an error occured grabbing environment variables:", err)
		os.Exit(1)
	}

	for _, v := range os.Environ() {
		fmt.Println(v)
	}
}
```
#### Added the environment variable
![](https://raw.githubusercontent.com/catmullet/envi/assets/envi_yamledit.png)
#### Run the app and check the results
![](https://raw.githubusercontent.com/catmullet/envi/assets/envi-test_results.png)

## Viewing envi.yaml
#### For anyone else without your key it's just encrypted garbage
![](https://raw.githubusercontent.com/catmullet/envi/assets/envi-encrypted.png)
#### For you use envi-cli edit or follow Google's documentation to decrypt with gcloud CLI (https://cloud.google.com/kms/docs/creating-keys)
