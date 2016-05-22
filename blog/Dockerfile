FROM debian:latest

RUN apt-get update
RUN apt-get install -y nagios-nrpe-server
RUN apt-get install -y nagios-nrpe-plugin

EXPOSE 5666

ENTRYPOINT ["/bin/bash", "-c", "/usr/sbin/nrpe -c /etc/nagios/nrpe.cfg -d;/bin/bash"]
