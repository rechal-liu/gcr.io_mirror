package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	ctx := context.Background()
	var (
		ghToken           = kingpin.Flag("github.token", "Github token.").Short('t').String()
		ghUser            = kingpin.Flag("github.user", "Github Owner.").Short('u').String()
		ghRepo            = kingpin.Flag("github.repo", "Github Repo.").Short('p').String()
		registry          = kingpin.Flag("docker.registry", "Docker Registry.").Short('r').Default("").String()
		registryNamespace = kingpin.Flag("docker.namespace", "Docker Registry Namespace.").Short('n').String()
		registryUserName  = kingpin.Flag("docker.user", "Docker Registry User.").Short('a').String()
		registryPassword  = kingpin.Flag("docker.secret", "Docker Registry Password.").Short('s').String()
		runId             = kingpin.Flag("github.run_id", "Github Run Id.").Short('i').String()
	)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	config := &Config{
		GhToken:           *ghToken,
		GhUser:            *ghUser,
		Repo:              *ghRepo,
		Registry:          *registry,
		RegistryNamespace: *registryNamespace,
		RegistryUserName:  *registryUserName,
		RegistryPassword:  *registryPassword,
		RunId:             *runId,
		Images:            []string{},
	}
	list, err := lineByLine("./images.txt")
	if err == nil {
		config.Images = list
	}
    fmt.Println(list)
	//docker login
	cli, ctx, err := dockerLogin(config)
	if err != nil {
		fmt.Printf("docker login 报错 : %s\n", err)
		os.Exit(0)
	}

	for _, image := range config.Images {
		originImageName := strings.TrimSpace(image)
		targetImageName := "cc237738572/" + strings.Join(strings.Split(originImageName, "/"), "_")

		fmt.Println("source:", originImageName, " , target:", targetImageName)
		//docker pull
		err := dockerPull(originImageName, cli, ctx)
		if err != nil {
			fmt.Printf("docker pull 报错： %s\n", err)
			os.Exit(0)
		}

		//docker tag
		err = dockerTag(originImageName, targetImageName, cli, ctx)
		if err != nil {
			fmt.Printf("docker tag 报错: %s\n", err)
			os.Exit(0)
		}
		//docker push
		err = dockerPush(targetImageName, cli, ctx, config)
		if err != nil {
			fmt.Printf("docker push 报错: %s\n", err)
			os.Exit(0)
		}

	}

}
func dockerLogin(config *Config) (*client.Client, context.Context, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, nil, err
	}
	fmt.Println("docker login, server: ", config.Registry, " user: ", config.RegistryUserName, ", password: ***")
	authConfig := types.AuthConfig{
		Username:      config.RegistryUserName,
		Password:      config.RegistryPassword,
		ServerAddress: config.Registry,
	}
	ctx := context.Background()
	_, err = cli.RegistryLogin(ctx, authConfig)
	if err != nil {
		return nil, nil, err
	}
	return cli, ctx, nil
}
func dockerPull(originImageName string, cli *client.Client, ctx context.Context) error {
	fmt.Println("docker pull ", originImageName)
	pullOut, err := cli.ImagePull(ctx, originImageName, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer pullOut.Close()
	io.Copy(os.Stdout, pullOut)
	return nil
}
func dockerTag(originImageName string, targetImageName string, cli *client.Client, ctx context.Context) error {
	fmt.Println("docker tag ", originImageName, " ", targetImageName)
	err := cli.ImageTag(ctx, originImageName, targetImageName)
	return err
}
func dockerPush(targetImageName string, cli *client.Client, ctx context.Context, config *Config) error {
	fmt.Println("docker push ", targetImageName)
	authConfig := types.AuthConfig{
		Username: config.RegistryUserName,
		Password: config.RegistryPassword,
	}
	if len(config.Registry) > 0 {
		authConfig.ServerAddress = config.Registry
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	pushOut, err := cli.ImagePush(ctx, targetImageName, types.ImagePushOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		return err
	}
	defer pushOut.Close()
	io.Copy(os.Stdout, pushOut)
	return nil
}

type Config struct {
	GhToken           string   `yaml:"gh_token"`
	GhUser            string   `yaml:"gh_user"`
	Repo              string   `yaml:"repo"`
	Registry          string   `yaml:"registry"`
	RegistryNamespace string   `yaml:"registry_namespace"`
	RegistryUserName  string   `yaml:"registry_user_name"`
	RegistryPassword  string   `yaml:"registry_password"`
	Images            []string `yaml:"images"`
	RunId             string   `yaml:"run_id"`
}

// lineByLine 逐行读取文本
func lineByLine(file string) ([]string, error) {

	var err error
	var list []string

	f, err := os.Open(file)
	if err != nil {
		return list, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Printf("error reading file %s", err)
			break
		}
		if !strings.HasPrefix(line, "#") {
			list = append(list, line)
		}

	}
	return list, nil
}
