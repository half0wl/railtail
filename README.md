# railtail

railtail is a simple TCP forwarder for Railway workloads connecting to
Tailscale nodes. It listens on a local address and forwards traffic it
receives on the local address to a target Tailscale peer address.

This was created to work around userspace networking restrictions. Dialing a
Tailscale node from a container requires you to do it over Tailscale's
local SOCKS5/HTTP proxy, which is not always ergonomical especially if
you're connecting to databases or other services with minimal support
for SOCKS5 (e.g. db connections from an application).

This is designed to be run as a separate service in Railway that you
connect to over Railway's Private Network.

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/template/YIGsfy?referralCode=EPXG5z)

## Configuration

| Environment Variable | CLI Argument   | Description                                                 |
| -------------------- | -------------- | ----------------------------------------------------------- |
| `TS_AUTH_KEY`        | N/A            | Required. Tailscale auth key. Must be set in environment.   |
| `TS_HOSTNAME`        | `-ts-hostname` | Required. Hostname to use for Tailscale.                    |
| `LISTEN_PORT`        | `-listen-port` | Required. Port to listen on.                                |
| `TARGET_ADDR`        | `-target-addr` | Required. Address of the Tailscale node to send traffic to. |

_CLI arguments will take precedence over environment variables._

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

2. Deploy railtail to Railway by clicking the button below:

   [![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/template/YIGsfy?referralCode=EPXG5z)

3. Use your new railtail service's Private Domain to connect to your RDS instance:

   ```sh
   DATABASE_URL="postgresql://u:p@${{railtail.RAILWAY_PRIVATE_DOMAIN}}:${{railtail.LISTEN_PORT}}/dbname"
   ```
