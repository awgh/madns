# madns: the DNS server for pentesters

## Dependencies & Requirements
 - [go language](https://golang.org/)
 - [go dns package](https://github.com/miekg/dns)
 - A domain you own


## Installation on Linux

### Install go
```
wget https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.17.5.linux-amd64.tar.gz
```

### Add go to your environment/PATH
```
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.profile
source ~/.profile
```

### Install madns (installs to ~/go/bin/madns)
```
go install github.com/awgh/madns@latest
```

### Create madns-config based off template
```
cp ~/go/pkg/mod/github.com/awgh/madns@*/madns-config.json.example ./madns-config.json
```

## Setup madns config

Edit the madns-config.json file, according to the following instructions.

### Port
Standard DNS port, only change if you know your setup differs.

`"Port": 53`

#### Dealing with systemd-resolved
If your system is running systemd-resolved (common for Ubuntu), you will have to follow these instructions to free up port 53: 
https://medium.com/@niktrix/getting-rid-of-systemd-resolved-consuming-port-53-605f0234f32f


### Handlers
This is where you define the domain/subdomain to trigger your email notification.

Each handler has a trigger portion, which describes the (sub)domains that it will handle, and **either** a Redirect command or a Respond command.  You must have a Redirect or a Respond command in each handler, but not both!

Additionally, handlers can have a NotifyEmail instruction, which notify you by email when the handler is invoked. They can also use the NotifySlack instruction, which sends the same notification to a Slack channel via webhooks.

**.** is the default DNS handler, if a query doesn't match any other handler it will use this handler.

#### Redirect handlers
Redirect commands will redirect the request to an upstream DNS server.  Redirect commands require the IP address and the port, like "8.8.8.8:53".

#### Respond handlers
Respond commands will respond with a fixed response.  Respond commands only need the IP address or the domain name (for a CNAME).  IP addresses can be either IPv4 or IPv6, and will generate an A/AAAA record accordingly.


### Examples
The following example is a catch-all handler that will redirect requests not handled by another handler to another DNS Server, in this case 8.8.8.8:

```
".": {
        "Redirect": "8.8.8.8:53"
        "NotifyEmail": "youremail@domain.com"
     },
```

Now you’ll want to create a subdomain that will trigger when a DNS lookup is performed on it for testing double blind XXE/SQLi/etc. It can be useful to setup an email with a +filterkeyword to make it easier to tell which handler fired when you get a successful hit.

In the following example, the triggering domain will always respond with a fixed address and also notify you of the hit by email:

```
"your.triggering.domain": { 
        "Respond": "192.168.1.1", 
        "NotifyEmail": "youremail+filterkeyword@domain.com"
        }
```

### SMTP Configuration (Optional)

If you want to use the NotifyEmail feature, you have to set the SMTP configuration values.

```
"SmtpUser":"yourburneremail@gmail.com",
"SmtpPassword":"<password to yourburneremail>",
"SmtpServer":"smtp.gmail.com:587",
"SmtpDelay":30,
```
The SmtpDelay parameter determines how many seconds madns will batch up alerts into a single email.  By default, this is set to 1 minute, so there will be a 1 minute delay before the first email is sent unless the SmtpDelay is set.

#### Gmail SMTP enable less-secure apps
So gmail does that whole security thing and won't let madns log in and
perform SMTP unless you enable less secure apps. https://www.google.com/settings/security/lesssecureapps

### Start madns
If you're listening to the default port 53 (or anything lower than 1024):

`sudo madns -c madns-config.json &`

For ports above 1024:

`madns -c madns-config.json &`

## Configure your domain
Add an subdomain record (an A record) in your DNS management section of your domain to point to the IP address that madns is running on. For example:

```
Type		Name			Value				TTL
A		<special>		<ip-to-madns-server>		7200
NS		<subdomain>		<special.domain>		7200
```
Also ensure that incoming/outgoing traffic on port 53 is open and outgoing SMTP traffic is allowed on your box.

## Test madns
Get the nameserver registered for your domain

`dig domain -t NS   `

Use that nameserver to query your subdomain

`dig @<nameserver.from.previous.dig> subdomain.domain -t NS`

If all is well you should see something like
```
;; QUESTION SECTION:
;<subdomain.domain.> IN    NS

;; AUTHORITY SECTION:
.<subdomain.domain>. 259200 IN NS   <special.domain.>
;; ADDITIONAL SECTION:
<special.domain.>          3600    IN      A       <ip.of.host.running.madns>
```


Now test with curl

`curl subdomain.subdomain.domain`

On the madns server you see notifications to stdout that it hit the Handler and sent an email such as:

`2017/09/21 11:24:37 sent email to xxe+dns@hotmail.com`

   

## systemd service file

You can set up madns to run as a systemd server which starts on boot.

Run the following commands to install madns in /opt and create a systemd service file for it.

```
sudo mkdir -p /opt/madns/
sudo cp ~/go/bin/madns /opt/madns/
sudo cp madns-config.json /opt/madns/
sudo nano /etc/systemd/system/madns.service
```

Put the following contents into the madns.service file and save it:
```
[Unit]
Description=madns DNS server
After=network.target

[Service]
WorkingDirectory=/opt/madns
ExecStart=/opt/madns/madns -c madns-config.json
ExecStop=/bin/kill $MAINPID
KillMode=process
Restart=on-failure
RestartSec=5s
Type=simple

[Install]
WantedBy=multi-user.target
Alias=madns.service
```

Finally, reload the systemd config files and start/enable madns:
```
sudo systemctl daemon-reload
sudo systemctl enable madns
sudo systemctl start madns
```
