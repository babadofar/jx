package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	cmdutil "github.com/jenkins-x/jx/pkg/jx/cmd/util"
	"github.com/jenkins-x/jx/pkg/kube"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/jenkins-x/jx/pkg/util"
	"gopkg.in/AlecAivazis/survey.v1"
	"k8s.io/client-go/kubernetes"
)

type EnvironmentOptions struct {
	CommonOptions

	Choose bool
}

const ()

var (
	environment_long = templates.LongDesc(`
		Displays or changes the current environment.`)
	environment_example = templates.Examples(`
		# view the current environment
		jx environment

		# view the current environment (concise version)
		jx env

		# Change the current environment to 'staging'
		jx env staging

		# Select which environment to change to from the available environments
		jx ns -s`)
)

func NewCmdEnvironment(f cmdutil.Factory, out io.Writer, errOut io.Writer) *cobra.Command {
	options := &EnvironmentOptions{
		CommonOptions: CommonOptions{
			Factory: f,
			Out:     out,
			Err:     errOut,
		},
	}
	cmd := &cobra.Command{
		Use:     "environment",
		Aliases: []string{"env"},
		Short:   "View or change the current environment in the current kubernetes cluster",
		Long:    environment_long,
		Example: environment_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.Choose, "select", "s", false, "Select which environment to switch to")
	return cmd
}

func (o *EnvironmentOptions) Run() error {
	kubeClient, currentNs, err := o.Factory.CreateClient()
	if err != nil {
		return err
	}
	jxClient, _, err := o.Factory.CreateJXClient()
	if err != nil {
		return err
	}

	devNs, currentEnv, err := kube.GetDevNamespace(kubeClient, currentNs)
	if err != nil {
		return err
	}
	envNames, err := kube.GetEnvironmentNames(jxClient, devNs)
	if err != nil {
		return err
	}

	config, po, err := kube.LoadConfig()
	if err != nil {
		return err
	}
	env := ""
	args := o.Args
	if len(args) > 0 {
		env = args[0]
	}
	if o.Choose {
		pick, err := kube.PickEnvironment(envNames)
		if err != nil {
			return err
		}
		env = pick
	}
	info := util.ColorInfo
	if env != "" {
		envResource, err := jxClient.JenkinsV1().Environments(devNs).Get(env, meta_v1.GetOptions{})
		if err != nil {
			return util.InvalidArg(env, envNames)
		}
		ns := envResource.Spec.Namespace
		if ns == "" {
			return fmt.Errorf("Cannot change to environment %s as it has no namespace!", env)
		}

		newConfig := *config
		ctx := kube.CurrentContext(config)
		if ctx == nil {
			return fmt.Errorf(noContextDefinedError)
		}
		if ctx.Namespace == ns {
			return nil
		}
		ctx.Namespace = ns
		err = clientcmd.ModifyConfig(po, newConfig, false)
		if err != nil {
			return fmt.Errorf("Failed to update the kube config %s", err)
		}
		fmt.Fprintf(o.Out, "Now using environment '%s' in tean '%s' on server '%s'.\n", info(env), info(devNs), info(kube.Server(config, ctx)))
	} else {
		ns := kube.CurrentNamespace(config)
		server := kube.CurrentServer(config)
		if env == "" {
			env = currentEnv
		}
		if env == "" {
			fmt.Fprintf(o.Out, "Usingnamespace '%s' from context named '%s' on server '%s'.\n", info(ns), info(config.CurrentContext), info(server))
		} else {
			fmt.Fprintf(o.Out, "Using environment '%s' in team '%s' on server '%s'.\n", info(env), info(devNs), info(server))
		}
	}
	return nil
}

func (o *EnvironmentOptions) PickNamespace(client *kubernetes.Clientset, defaultNamespace string) (string, error) {
	list, err := client.CoreV1().Namespaces().List(meta_v1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("Failed to load current namespaces %s", err)
	}
	names := []string{}
	for _, n := range list.Items {
		names = append(names, n.Name)
	}
	var qs = []*survey.Question{
		{
			Name: "namespace",
			Prompt: &survey.Select{
				Message: "Change namespace: ",
				Options: names,
				Default: defaultNamespace,
			},
		},
	}
	answers := struct {
		Namespace string
	}{}
	err = survey.Ask(qs, &answers)
	if err != nil {
		return "", err
	}
	return answers.Namespace, nil
}
