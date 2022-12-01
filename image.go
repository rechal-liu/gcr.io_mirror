package main
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"os"
	"strings"
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
		Images: map[string]string{
			"registry.gitlab.com/gitlab-org/gitlab-runner:alpine-v15.6.1":          "gitlab-runner:alpine-v15.6.1",
		},
	}
	needImagesFile, err := ioutil.ReadFile("needImages.yaml")
	if err == nil {
		rules := make(map[string]string)
		err2 := yaml.Unmarshal(needImagesFile, &rules)
		if err2 == nil {
			config.Images = rules
		}
	}

	//docker login
	cli, ctx, err := dockerLogin(config)
	if err != nil {
		fmt.Printf("docker login 报错 : %s\n", err)
		os.Exit(0)
	}
	

	for k, v := range config.Images {
		originImageName := k
		targetImageName := strings.ToLower(config.RegistryNamespace) + "/" + v

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
	GhToken           string            `yaml:"gh_token"`
	GhUser            string            `yaml:"gh_user"`
	Repo              string            `yaml:"repo"`
	Registry          string            `yaml:"registry"`
	RegistryNamespace string            `yaml:"registry_namespace"`
	RegistryUserName  string            `yaml:"registry_user_name"`
	RegistryPassword  string            `yaml:"registry_password"`
	Images             map[string]string `yaml:"images"`
	RunId             string            `yaml:"run_id"`
}