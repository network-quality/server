# Build, install and run the go implementation of an RPM server.
# Build with: docker build -t rpmserver .

FROM golang:1.18.2-alpine3.15

RUN mkdir /server

# Be selective about the files we add in to the container.
ADD *.go /server
ADD go.mod /server
ADD go.sum /server
ADD config.json.in /server

# Set the working directory
WORKDIR /server

# Now build.
RUN go mod download
RUN go build -o networkqualityd ./networkqualityd.go ./announcer.go

# Configure default values that a user can override.
ENV cert_file=/live/fullchain.pem
ENV key_file=/live/privkey.pem
ENV public_name=networkquality.example.com
ENV config_name=networkquality.example.com
ENV config_port=4043
ENV listen_addr=0.0.0.0
ENV template=config.json.in

# By default, this is what the container will run when `docker run`
# is issued by a user.
CMD /server/networkqualityd -cert-file ${cert_file} -key-file ${key_file} -public-name ${public_name} -config-port ${config_port} -config-name ${config_name} ${debug} -listen-addr ${listen_addr} -template ${template}
