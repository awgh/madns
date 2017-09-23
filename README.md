# madns
DNS server for pentesters

### Pre-reqs
 - [go language](https://golang.org/)
 - [go dns package](https://github.com/miekg/dns)
 - domain you own

### Instructions for Ubuntu 16.04

#### Install go
```
curl https://storage.googleapis.com/golang/go1.9.linux-amd64.tar.gz > go1.9.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.9.linux-amd64.tar.gz
```
#### Add go to your environment/PATH
```
sudo echo -ne "export GOPATH=$HOME/go\nexport PATH=$PATH:/usr/local/go/bin" >> /etc/profile
source /etc/profile
```
#### Verify go is in your path
`echo $PATH $GOPATH`

#### Create go build directories
`mkdir -p $HOME/go/src/`

##### Download madns
```
cd $HOME/go/src
git clone https://github.com/awgh/madns.git
```
#### Grab DNS go package dependency
`go get -v github.com/miekg/dns`

#### Build madns
`go build`

#### Create madns-config based off template
`cp madns-config.example.json madns-config.json`

### Setup madns config

#### SMTP
```
"SmtpUser":"yourburneremail@gmail.com",
"SmtpPassword":"<password to yourburneremail>",
"SmtpServer":"smtp.gmail.com:587",
```
#### Port
Standard DNS port, only change if you know your setup differs.
`"Port": 53`

#### Handlers
This is where you define the domain/subdomain to trigger your email notification.

. is the default DNS handler, if a query doesn't match any other handler it will use this 

```
".": {
        "Redirect": "” // used to forward traffic to another DNS server
        "NotifyEmail": "" // the email address to notify when this handler is invoked
     },
```
Now you’ll want to create a subdomain that will trigger when a DNS lookup is performed on it for testing double blind XXE/SQLi/etc. It can be useful to setup an email with a +filterkeyword to make it easier to alert you when you’ve got a successful hit.
```
"your.triggering.domain": { 
        "Redirect": "",
        "NotifyEmail": "youremail+filterkeyword@domain.com"
        }
```
#### Gmail SMTP enable less-secure apps
So gmail does that whole security thing and won't let madns log in and
perform SMTP unless you enable less secure apps. https://www.google.com/settings/security/lesssecureapps

#### Start madns
`sudo ./madns &`

#### Configure your domain
Add an subdomain record (an A record) in your DNS management section of your domain to point to the IP address that madns is running on. For example:

```
Type		Name			Value				TTL
A		<special>		<ip-to-madns-server>		7200
NS		<subdomain>		<special.domain>		7200
```
Also ensure that incoming/outgoing traffic on port 53 is open and outgoing SMTP traffic is allowed on your box.

#### Test madns
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
**TODO** Add notes pertaining to static address response when feature is added

Now test with curl
`curl subdomain.subdomain.domain`

On the madns server you see notifications to stdout that it hit the Handler and sent an email such as:

`2017/09/21 11:24:37 sent email to xxe+dns@hotmail.com`

   


