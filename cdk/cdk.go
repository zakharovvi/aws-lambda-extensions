package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

func NewCdkStack(scope constructs.Construct, id string, props awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, &props)

	// https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services#adding-the-identity-provider-to-aws
	provider := awsiam.NewOpenIdConnectProvider(
		stack,
		jsii.String("github-openid"),
		&awsiam.OpenIdConnectProviderProps{
			Url:         jsii.String("https://token.actions.githubusercontent.com"),
			ClientIds:   jsii.Strings("sts.amazonaws.com"),
			Thumbprints: jsii.Strings("6938fd4d98bab03faadb97b34396831e3780aea1"),
		},
	)

	bucket := awss3.NewBucket(stack, jsii.String("sam-releases-bucket"), &awss3.BucketProps{
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		EnforceSSL:        jsii.Bool(true),
	})

	// https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/sam-permissions.html
	policies := &[]awsiam.IManagedPolicy{
		awsiam.NewManagedPolicy(stack, jsii.String("sam-deploy-policy"), &awsiam.ManagedPolicyProps{
			Statements: &[]awsiam.PolicyStatement{
				awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
					Effect:    awsiam.Effect_ALLOW,
					Actions:   jsii.Strings("s3:GetObject", "s3:PutObject"),
					Resources: jsii.Strings(*bucket.ArnForObjects(jsii.String("*"))),
				}),
			},
		}),
		awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("AWSLambda_FullAccess")),
		awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("AWSCloudFormationFullAccess")),
		awsiam.ManagedPolicy_FromManagedPolicyArn(stack, jsii.String("invoke-policy"), jsii.String("arn:aws:iam::aws:policy/service-role/AWSLambdaRole")),
	}

	githubActionsRole := awsiam.NewRole(stack, jsii.String("github-actions-role"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewFederatedPrincipal(
			provider.OpenIdConnectProviderArn(),
			&map[string]interface{}{
				"StringEquals": interface{}(map[string]string{
					"token.actions.githubusercontent.com:sub": "repo:zakharovvi/aws-lambda-extensions:ref:refs/heads/main",
					"token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
				}),
			},
			jsii.String("sts:AssumeRoleWithWebIdentity"),
		),
		ManagedPolicies: policies,
	})

	localTestingRole := awsiam.NewRole(stack, jsii.String("local-testing-role"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewPrincipalWithConditions(
			awsiam.NewAccountRootPrincipal(),
			&map[string]interface{}{
				"Bool": interface{}(map[string]string{
					"aws:MultiFactorAuthPresent": "true",
				}),
			},
		),
		ManagedPolicies: policies,
	})

	awscdk.NewCfnOutput(stack, jsii.String("bucket-name"), &awscdk.CfnOutputProps{
		Value:       bucket.BucketName(),
		Description: jsii.String("s3 bucket for samconfig.toml"),
		ExportName:  jsii.String("SAMReleasesS3Bucket"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("github-actions-role-arn"), &awscdk.CfnOutputProps{
		Value:       githubActionsRole.RoleArn(),
		Description: jsii.String("role to use in github actions aws-actions/configure-aws-credentials@v1"),
		ExportName:  jsii.String("GithubActionsRoleARN"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("local-testing-role-arn"), &awscdk.CfnOutputProps{
		Value:       localTestingRole.RoleArn(),
		Description: jsii.String("role to use for sam local testing"),
		ExportName:  jsii.String("LocalTestingRoleARN"),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewCdkStack(
		app,
		"ci-aws-lambda-extensions",
		awscdk.StackProps{
			Description: jsii.String("Resources to run integration tests for zakharovvi/aws-lambda-extensions project in github actions"),
			Tags: &map[string]*string{
				"project": jsii.String("zakharovvi/aws-lambda-extensions"),
			},
		},
	)

	app.Synth(nil)
}
