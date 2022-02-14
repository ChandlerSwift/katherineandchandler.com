# Katherine and Chandler

I'm getting married!

### Deploying
Create user `weddingsite` on server. Deploy the database.

Set up a systemd file in `/etc/systemd/system/weddingsite.service` (filling in
`ADMIN_PASSWORD` as needed):
```ini
[Unit]
Description=katherineandchandler.com
After=network.target

[Service]
Environment="ADMIN_PASSWORD=..."
ExecStart=/home/weddingsite/katherineandchandler.com
WorkingDirectory=/home/weddingsite
User=weddingsite
Group=weddingsite

[Install]
WantedBy=multi-user.target
```

and run `make deploy`
