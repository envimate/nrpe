FROM debian:latest

RUN apt-get update
RUN apt-get install -y nagios-nrpe-server
RUN apt-get install -y nagios-nrpe-plugin

EXPOSE 5666

ENTRYPOINT ["/bin/bash", "-c", "/etc/init.d/nagios-nrpe-server start;/bin/bash"]
