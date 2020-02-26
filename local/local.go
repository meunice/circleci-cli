package local

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"syscall"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

var picardRepo = "circleci/picard"

const DefaultConfigPath = ".circleci/config.yml"

type BuildOptions struct {
	Cfg  *settings.Config
	Args []string
	Help func() error
}

type buildAgentSettings struct {
	LatestSha256 string
}

func UpdateBuildAgent() error {
	latestSha256, err := findLatestPicardSha()

	if err != nil {
		return err
	}

	fmt.Printf("Latest build agent is version %s\n", latestSha256)

	return nil
}

func Execute(opts BuildOptions) error {
	for _, f := range opts.Args {
		if f == "--help" || f == "-h" {
			return opts.Help()
		}
	}

	processedArgs, err := extractConfigPath(opts.Args)

	if err != nil {
		return err
	}

	cl := client.NewClient(opts.Cfg.Host, opts.Cfg.Endpoint, opts.Cfg.Token, opts.Cfg.Debug)

	configResponse, err := api.ConfigQuery(cl, processedArgs.configPath)

	if err != nil {
		return err
	}

	if !configResponse.Valid {
		return fmt.Errorf("config errors %v", configResponse.Errors)
	}

	pwd, err := os.Getwd()

	if err != nil {
		return errors.Wrap(err, "Could not find pwd")
	}

	if err = ensureDockerIsAvailable(); err != nil {
		return err
	}

	f, err := ioutil.TempFile("/tmp", "*-config.yml")

	if err != nil {
		return errors.Wrap(err, "Error creating temporary config file")
	}

	if _, err = f.WriteString(configResponse.OutputYaml); err != nil {
		return errors.Wrap(err, "Error writing processed config to temporary file")
	}

	image, err := picardImage()

	if err != nil {
		return errors.Wrap(err, "Could not find picard image")
	}

	configPathInsideContainer := "/tmp/local_build_config.yml"

	arguments := []string{"docker", "run", "--interactive", "--tty", "--rm",
		"--volume", "/var/run/docker.sock:/var/run/docker.sock",
		"--volume", fmt.Sprintf("%s:%s", f.Name(), configPathInsideContainer),
		"--volume", fmt.Sprintf("%s:%s", pwd, pwd),
		"--volume", fmt.Sprintf("%s:/root/.circleci", circleCiDir()),
		"--workdir", pwd,
		image, "circleci", "build", "--config", configPathInsideContainer}

	arguments = append(arguments, processedArgs.args...)

	if opts.Cfg.Debug {
		_, err = fmt.Fprintf(os.Stderr, "Starting docker with args: %s", arguments)
		if err != nil {
			return err
		}
	}

	dockerPath, err := exec.LookPath("docker")

	if err != nil {
		return errors.Wrap(err, "Could not find a `docker` executable on $PATH; please ensure that docker installed")
	}

	err = syscall.Exec(dockerPath, arguments, os.Environ()) // #nosec
	return errors.Wrap(err, "failed to execute docker")
}

type processedArgs struct {
	args       []string
	configPath string
}

func extractConfigPath(args []string) (*processedArgs, error) {
	flagSet := pflag.NewFlagSet("mock", pflag.ContinueOnError)
	var configFilename string
	flagSet.StringVarP(&configFilename, "config", "c", DefaultConfigPath, "config file")
	if err := flagSet.Parse(args); err != nil {
		return nil, err
	}
	return &processedArgs{
		configPath: configFilename,
		args:       flagSet.Args(),
	}, nil
}

func picardImage() (string, error) {

	sha := loadCurrentBuildAgentSha()

	if sha == "" {

		fmt.Println("Downloading latest CircleCI build agent...")

		var err error

		sha, err = findLatestPicardSha()

		if err != nil {
			return "", err
		}

	}
	fmt.Printf("Docker image digest: %s\n", sha)
	return fmt.Sprintf("%s@%s", picardRepo, sha), nil
}

func ensureDockerIsAvailable() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("could not find `docker` on the PATH; please ensure that docker is installed")
	}

	dockerRunning := exec.Command("docker", "version").Run() == nil // #nosec

	if !dockerRunning {
		return errors.New("failed to connect to docker; please ensure that docker is running, and that `docker version` succeeds")
	}

	return nil
}

// Still depends on a function in cmd/build.go
func findLatestPicardSha() (string, error) {

	if err := ensureDockerIsAvailable(); err != nil {
		return "", err
	}

	outputBytes, err := exec.Command("docker", "pull", picardRepo).CombinedOutput() // #nosec

	if err != nil {
		return "", errors.Wrap(err, "failed to pull latest docker image")
	}

	output := string(outputBytes)
	sha256 := regexp.MustCompile("(?m)sha256:[0-9a-f]+")
	latest := sha256.FindString(output)

	if latest == "" {
		return "", fmt.Errorf("failed to parse sha256 from docker pull output")
	}

	// This function still lives in cmd/build.go
	err = storeBuildAgentSha(latest)

	if err != nil {
		return "", err
	}

	return latest, nil
}

func circleCiDir() string {
	return path.Join(settings.UserHomeDir(), ".circleci")
}

func buildAgentSettingsPath() string {
	return path.Join(circleCiDir(), "build_agent_settings.json")
}

func storeBuildAgentSha(sha256 string) error {
	settings := buildAgentSettings{
		LatestSha256: sha256,
	}

	settingsJSON, err := json.Marshal(settings)

	if err != nil {
		return errors.Wrap(err, "Failed to serialize build agent settings")
	}

	if err = os.MkdirAll(circleCiDir(), 0700); err != nil {
		return errors.Wrap(err, "Could not create settings directory")
	}

	err = ioutil.WriteFile(buildAgentSettingsPath(), settingsJSON, 0644)

	return errors.Wrap(err, "Failed to write build agent settings file")
}

func loadCurrentBuildAgentSha() string {
	if _, err := os.Stat(buildAgentSettingsPath()); os.IsNotExist(err) {
		return ""
	}

	settingsJSON, err := ioutil.ReadFile(buildAgentSettingsPath())

	if err != nil {
		_, er := fmt.Fprint(os.Stderr, "Failed to load build agent settings JSON", err.Error())
		if er != nil {
			panic(er)
		}

		return ""
	}

	var settings buildAgentSettings

	err = json.Unmarshal(settingsJSON, &settings)

	if err != nil {
		_, er := fmt.Fprint(os.Stderr, "Failed to parse build agent settings JSON", err.Error())
		if er != nil {
			panic(er)
		}

		return ""
	}

	return settings.LatestSha256
}
