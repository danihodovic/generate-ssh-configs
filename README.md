# generate-ssh-configs

<p align="center"><img src="./example.gif?raw=true"/></p>

## Description

generate-ssh-configs reads cloud providers API and generates ssh config files
for you. This is especially useful when dealing with tens or hundreds of
servers.

The program writes to stdout. Using shell redirection we can write persistent
config files and include them using the ssh `Include` directive.

## Examples

### Prerequisites
**Install generate-ssh-configs**
```
go get github.com/danihodovic/generate-ssh-configs
```

#### Ensure your ssh config includes all the config files in the ssh directory.
```
cat ~/.ssh/.config
# ...at the bottom of the file...
Include ~/.ssh/config-*
```

#### Ensure your AWS credentials have been configured if using AWS

See https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html

#### Ensure `$DIGITAL_OCEAN_TOKEN` is set if using DigitalOcean

See https://www.digitalocean.com/docs/api/create-personal-access-token/

### Generate ssh configs for all AWS instances
Uses the current AWS region (`AWS_DEFAULT_REGION`) and generates all
configs using the EC2 API.
```
generate-ssh-configs aws --prefix myservers --user myuser > ~/.ssh/config-myservers
```

### AWS multi-region, multi-environment setup
Using multiple regions, environments and jumphosts for each region and
environment. This works if all of your environments are contained in a single
AWS account and separated by VPC and tags.
```
# Generate configs for dev,test,prod in eu-west-1
AWS_DEFAULT_REGION=eu-west-1 generate-ssh-configs aws \
    --prefix myorg-dev-eu-west-1 \
    --filters 'Name=tag:Environment,Values=dev'
    --jumphost jumphost --user dani \
    > ~/.ssh/config-myorg-dev-eu-west-1

AWS_DEFAULT_REGION=eu-west-1 generate-ssh-configs aws \
    --prefix myorg-prod-eu-west-1 \
    --filters 'Name=tag:Environment,Values=prod' \
    --jumphost jumphost \
    --user dani  \
    > ~/.ssh/config-myorg-prod-eu-west-1


# Generate configs for dev,test,prod in ap-south 1
AWS_DEFAULT_REGION=ap-south-1 generate-ssh-configs aws \
    --prefix myorg-dev-ap-south-1 \
    --filters 'Name=tag:Environment,Values=dev' \
    --jumphost jumphost \
    --user dani \
    > ~/.ssh/config-myorg-dev-ap-south-1

AWS_DEFAULT_REGION=ap-south-1 generate-ssh-configs aws \
    --prefix myorg-prod-ap-south-1 \
    --filters 'Name=tag:Environment,Values=prod' \
    --jumphost jumphost \
    --user dani  \
    > ~/.ssh/config-myorg-prod-ap-south-1
```

## Usage with [FZF](https://github.com/junegunn/fzf)
SSH configs work beautifully with FZF since the servers are essentially a list.
Using some bash magic we can quickly to select the server we want to ssh to.

Here is an example of using fzf and zsh to quickly select a server. Pressing
Ctrl+s in a terminal launches fzf-ssh. Place the script in your `~/.zshrc`
```
stty stop undef
function fzf-ssh {
  all_matches=$(grep -P -r "Host\s+\w+" ~/.ssh/ | grep -v '\*')
  only_host_parts=$(echo "$all_matches" | awk '{print $NF}')
  selection=$(echo "$only_host_parts" | fzf)
  echo $selection

  if [ ! -z $selection ]; then
    BUFFER="ssh $selection"
    zle accept-line
  fi
  zle reset-prompt
}
zle     -N     fzf-ssh
bindkey "^s" fzf-ssh
```

## Features
- **AWS**
  - Uses name tags to identify instances.
  - Works with jumphosts or bastion hosts.
  - Uses the public IP if
    - the instance is in a public subnet
    - the security group allows ingress port 22 from the public internet
    - the security group allows ingress port 22 from subnet provided via `--subnet` flag
  - Otherwise it uses the private IP and routes through the jumphost if one is
    configured.
- **DigitalOcean**
