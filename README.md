# <img align="left" width="40" height="40" src="https://res.cloudinary.com/railway/image/upload/v1734036971/railtail_avdaue.png" alt="railtail logo"> railtail

railtail is a HTTP/TCP proxy for Railway workloads connecting to Tailscale
nodes. It listens on a local address and forwards traffic it receives on
the local address to a target Tailscale node address.

📣 This is a workaround until there are [full VMs available in Railway](https://help.railway.com/feedback/full-unix-v-ms-44eef294). Please upvote the thread if you want this feature!

## Usage

1. [Install and setup Tailscale](https://tailscale.com/kb/1017/install) on the
   machine you want to connect to. If you're using Tailscale as a subnet
   router, ensure you advertise the correct routes and approve the subnets
   in the Tailscale admin console.

2. Deploy this template to Railway:

   [![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/template/railtail?referralCode=EPXG5z)

3. In services that need to connect to the Tailscale node, connect to your
   railtail service using the `RAILWAY_PRIVATE_DOMAIN` and `LISTEN_PORT`
   variables. For example:

   ```sh
   MY_PRIVATE_TAILSCALE_SERVICE="http://{{railtail.RAILWAY_PRIVATE_DOMAIN}}:${{railtail.LISTEN_PORT}}"
   ```

Look at the [Examples](#examples) section for provider-specific examples.

## Configuration

railtail will forward TCP connections if you provide a `TARGET_ADDR` without
a `http://` or `https://` scheme. If you want railtail to act as an HTTP
proxy, ensure you have a `http://` or `https://` in your `TARGET_ADDR`.

| Environment Variable | CLI Argument       | Description                                                                                                                                                   |
| -------------------- | ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TARGET_ADDR`        | `-target-addr`     | Required. Address of the Tailscale node to send traffic to.                                                                                                   |
| `LISTEN_PORT`        | `-listen-port`     | Required. Port to listen on.                                                                                                                                  |
| `TS_HOSTNAME`        | `-ts-hostname`     | Required. Hostname to use for Tailscale.                                                                                                                      |
| `TS_AUTH_KEY`        | N/A                | Required. Tailscale auth key. Must be set in environment.                                                                                                     |
| `TS_LOGIN_SERVER`    | `-ts-login-server` | Optional. Base URL of the control server. If you are using Headscale for your control server, use your Headscale instance's url. Defaults to using Tailscale. |
| `TS_STATEDIR_PATH`   | `-ts-state-dir`    | Optional. Tailscale state dir. Defaults to `/tmp/railtail`.                                                                                                   |

_CLI arguments will take precedence over environment variables._

## About

This was created to work around userspace networking restrictions. Dialing a
Tailscale node from a container requires you to do it over Tailscale's
local SOCKS5/HTTP proxy, which is not always ergonomical especially if
you're connecting to databases or other services with minimal support
for SOCKS5 (e.g. db connections from an application).

railtail is designed to be run as a separate service in Railway that you
connect to over Railway's Private Network.

> ⚠️ **Warning**: Do not expose this service on Railway publicly!
>
> ![Networking settings warning](https://res.cloudinary.com/railway/image/upload/v1733851092/cs-2024-12-11-01.12_f1z1xy.png)
>
> This service is intended to be used via Railway's Private Network only.

## Examples

### Connecting to an AWS RDS instance

1. Configure Tailscale on an EC2 instance in the same VPC as your RDS instance:

   ```sh
   # In EC2
   curl -fsSL https://tailscale.com/install.sh | sh

   # Enable IP forwarding
   echo 'net.ipv4.ip_forward = 1' | sudo tee -a /etc/sysctl.d/99-tailscale.conf
   echo 'net.ipv6.conf.all.forwarding = 1' | sudo tee -a /etc/sysctl.d/99-tailscale.conf
   sudo sysctl -p /etc/sysctl.d/99-tailscale.conf

   # Start Tailscale. Follow instructions to authenticate the node if needed,
   # and make sure you approve the subnet routes in the Tailscale admin console
   sudo tailscale up --reset --advertise-routes=172.31.0.0/16
   ```

2. Deploy railtail into your pre-existing Railway project:

   [![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/template/railtail?referralCode=EPXG5z)

3. Use your new railtail service's Private Domain to connect to your RDS instance:

   ```sh
   DATABASE_URL="postgresql://username:password@${{railtail.RAILWAY_PRIVATE_DOMAIN}}:${{railtail.LISTEN_PORT}}/dbname"
   ```
