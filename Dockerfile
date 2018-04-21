FROM alpine

ADD _output/bin/anthill /usr/local/bin/anthill

CMD ["/usr/local/bin/anthill", "--logtostderr"]
