// Integration test to deploy all examples and invoke lambda functions
package main_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/pelletier/go-toml/v2"
)

var (
	cloudformationClient *cloudformation.Client
	lambdaClient         *lambda.Client
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Panic(err)
	}

	cloudformationClient = cloudformation.NewFromConfig(cfg)
	lambdaClient = lambda.NewFromConfig(cfg)
}

func TestLambda(t *testing.T) {
	examplesPath, err := filepath.Abs("../../examples")
	if err != nil {
		t.Fatal(err)
	}
	exampleDirs, err := os.ReadDir(examplesPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, exampleDir := range exampleDirs {
		if !exampleDir.IsDir() {
			t.Fatalf("unexpected file in examples directory: %s\n", exampleDir.Name())
		}
		t.Run(exampleDir.Name(), func(t *testing.T) {
			exampleFullPath := filepath.Join(examplesPath, exampleDir.Name())

			if err := sam(t, exampleFullPath); err != nil {
				t.Fatalf("sam: %s\n", err)
			}

			stackName, err := getStackName(t, exampleFullPath)
			if err != nil {
				t.Fatalf("getStackName: %s\n", err)
			}

			functionARN, err := getFunctionARN(t, stackName)
			if err != nil {
				t.Fatalf("getFunctionARN: %s\n", err)
			}

			res, err := lambdaClient.Invoke(context.Background(), &lambda.InvokeInput{
				FunctionName: &functionARN,
				LogType:      types.LogTypeTail,
			})
			if err != nil {
				t.Fatalf("lambdaClient.Invoke: %s\n", err)
			}

			logs, err := base64.StdEncoding.DecodeString(*res.LogResult)
			if err != nil {
				t.Logf("could not decode logs: %s", err)
			}
			t.Logf("function logs: %s", logs)

			t.Logf("function response: %s", res.Payload)
			if res.FunctionError != nil {
				t.Fatalf("function invocation error: %s\n", *res.FunctionError)
			}
		})
	}
}

func sam(t *testing.T, exampleFullPath string) error {
	t.Helper()

	validateCmd := exec.Command("sam", "validate")
	validateCmd.Dir = exampleFullPath
	b, err := validateCmd.CombinedOutput()
	t.Logf("validate: %s", b)
	if err != nil {
		return fmt.Errorf("sam validate: %s\n", err)
	}

	buildCmd := exec.Command("sam", "build")
	buildCmd.Dir = exampleFullPath
	goWorkFullPath, err := filepath.Abs("../../go.work")
	if err != nil {
		return fmt.Errorf("goWorkFullPath: %s\n", err)
	}
	buildCmd.Env = append(os.Environ(), fmt.Sprintf("GOWORK=%s", goWorkFullPath))
	b, err = buildCmd.CombinedOutput()
	t.Logf("build: %s", b)
	if err != nil {
		return fmt.Errorf("sam build: %s\n", err)
	}

	deployCmd := exec.Command("sam", "deploy", "--no-confirm-changeset", "--no-fail-on-empty-changeset")
	deployCmd.Dir = exampleFullPath
	b, err = deployCmd.CombinedOutput()
	t.Logf("deploy: %s", b)
	if err != nil {
		return fmt.Errorf("sam deploy: %s\n", err)
	}
	return nil
}

type SAMConfig struct {
	Default Default
}
type Default struct {
	Deploy Deploy
}
type Deploy struct {
	Parameters Parameters
}
type Parameters struct {
	StackName string `toml:"stack_name"`
}

func getStackName(t *testing.T, exampleFullPath string) (string, error) {
	t.Helper()

	samConfigPath := filepath.Join(exampleFullPath, "samconfig.toml")
	var cfg SAMConfig
	file, err := os.Open(samConfigPath)
	if err != nil {
		return "", err
	}
	if err := toml.NewDecoder(file).Decode(&cfg); err != nil {
		return "", err
	}
	if cfg.Default.Deploy.Parameters.StackName == "" {
		return "", fmt.Errorf("could not find stack_name field in config %s", samConfigPath)
	}
	return cfg.Default.Deploy.Parameters.StackName, nil
}

func getFunctionARN(t *testing.T, stackName string) (string, error) {
	t.Helper()

	stacks, err := cloudformationClient.DescribeStacks(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return "", err
	}
	if len(stacks.Stacks) != 1 {
		return "", fmt.Errorf("could not find stack %s", stackName)
	}
	for _, output := range stacks.Stacks[0].Outputs {
		if *output.OutputKey == "ExampleFunction" {
			if !arn.IsARN(*output.OutputValue) {
				return "", fmt.Errorf("output value for key ExampleFunction is not valid ARN: %s in stack %s", *output.OutputValue, stackName)
			}
			return *output.OutputValue, nil
		}
	}
	return "", fmt.Errorf("could not find ExampleFunction output key in stack %s", stackName)
}
