#!/bin/bash
ARGS=`getopt -o "hc:k:p:" -l "help,controller:,key:,path:" -n "hello.sh" -- "$@"`

eval set -- "${ARGS}"

DEFAULT_PATH="/opt/"
PACKAGE_NAME="smartcmp-gateway.tar.gz"


function open_firewall_port() {
    PORT_LIST="9090 8500 4822"
    state=$(firewall-cmd --state)
    if [ "$state" != "running" ]; then
        systemctl restart firewalld
    fi
    for port in $PORT_LIST
    do
        firewall-cmd --zone=public --add-port=${port}/tcp --permanent
    done
    firewall-cmd --reload
    systemctl restart firewalld
}

function config_guacd() {
    GUACD_PATH=$1
    sudo yum localinstall ${GUACD_PATH}/deps/*rpm -y
    sudo yum localinstall ${GUACD_PATH}/guacd/*rpm -y
    sudo echo OPTS="-l 4822 -b 0.0.0.0" >> ${GUACD_PATH}/guacd/guacd_config
}

function create_service_config() {
    # create prometheus service file
    SERVER_PATH=$1
    sudo cat > /usr/lib/systemd/system/prometheus.service <<EOF
[Unit]
Description=Prometheus
Wants=network-online.target
After=network-online.target

[Service]
User=gateway
Group=gateway
Type=simple
ExecStart=${SERVER_PATH}/prometheus/prometheus \
--config.file ${SERVER_PATH}/prometheus/prometheus.yml \
--storage.tsdb.path ${SERVER_PATH}/prometheus/data/ \
--web.console.templates=${SERVER_PATH}/prometheus/consoles \
--web.console.libraries=${SERVER_PATH}/prometheus/console_libraries
ExecStop=/bin/kill -15 $MAINPID

LimitNOFILE=102400

[Install]
WantedBy=multi-user.target
EOF
    # create consul service file
    IP=$(ip addr|grep inet|grep -v 127.0.0.1|grep -v inet6|awk 'NR==1{print gensub(/(.*)\/(.*)/, "\\1", "g", $2)}'|tr -d "addr:")
    if [ ! -n "$IP" ]; then
        echo "The host IP could not be obtained!"
    fi
    sudo cat > /usr/lib/systemd/system/consul.service <<EOF
[Unit]
Description=consul agent
Requires=network-online.target
After=network-online.target
[Service]
User=gateway
Group=gateway
Restart=on-failure
ExecStart=${SERVER_PATH}/consul/consul agent \
    -server -ui -client=0.0.0.0 \
    -bind=$IP \
    -data-dir=${SERVER_PATH}/consul/data -config-dir=${SERVER_PATH}/consul/consul.d -bootstrap-expect=1
ExecStop=/bin/kill -15 \$MAINPID
Type=simple
[Install]
WantedBy=multi-user.target
EOF

    # create guacd service
    sudo cat > /usr/lib/systemd/system/guacd.service <<EOF
[Unit]
Description=Guacamole Proxy Service
Documentation=man:guacd(8)
After=network.target

[Service]
EnvironmentFile=-${SERVER_PATH}/guacd/guacd_config
Environment=HOME=${SERVER_PATH}/guacd/data
ExecStart=/usr/local/sbin/guacd -f \$OPTS
ExecStop=/bin/kill -15 \$MAINPID
Restart=on-failure
User=gateway
Group=gateway

LimitNOFILE=102400

[Install]
WantedBy=multi-user.target
EOF

    # create proxy config file
    CONTROLLER_ADDRESS=$2
    CLIENT_KEY=$3
    sudo cat > $SERVER_PATH/proxy/smartcmp-proxy-agent.config <<EOF
CONTROLLER_ADDRESS: "${CONTROLLER_ADDRESS}"
CLIENT_KEY: "${CLIENT_KEY}"
LOG_PATH: "${SERVER_PATH}/proxy/"
DEFAULT_SERVICE:
  PROMETHEUS: "127.0.0.1:9090"
  CONSUL: "127.0.0.1:8500"
  GUACD: "127.0.0.1:4822"
EOF
    sudo cat >${SERVER_PATH}/proxy/smartcmp-proxy-agent.env << EOF
PROXY_CONFIG_PATH=${SERVER_PATH}/proxy/smartcmp-proxy-agent.config
EOF

    # create proxy service file
    sudo cat > /usr/lib/systemd/system/smartcmp-proxy-agent.service <<EOF
[Unit]
Description=SmartCMP Proxy Service

[Service]
Restart=always
RestartSec=10s

EnvironmentFile=-${SERVER_PATH}/proxy/smartcmp-proxy-agent.env
User=gateway
Group=gateway
ExecStart=${SERVER_PATH}/proxy/smartcmp-proxy-agent
ExecStop=/bin/kill -15 \$MAINPID

LimitNOFILE=102400

[Install]
WantedBy=multi-user.target
EOF

}

function make_file() {
    CONTROLLER_ADDRESS=$1
    CLIENT_KEY=$2
    user_base_path=$3

    if [[ ! $CONTROLLER_ADDRESS =~ "platform-api" ]]; then
        CONTROLLER_ADDRESS="${CONTROLLER_ADDRESS}/platform-api"
    fi

    if [ -n "$user_base_path" ]; then
        BASE_PATH="$user_base_path/smartcmp-gateway"
    else
        BASE_PATH="$DEFAULT_PATH/smartcmp-gateway"
    fi
    echo "BASE_PATH: $BASE_PATH"
    if [ ! -d ${BASE_PATH} ]; then
        sudo mkdir -p ${BASE_PATH}
    fi
    sudo mkdir -p ${BASE_PATH}/consul/data
    sudo mkdir -p ${BASE_PATH}/consul/consul.d
    sudo mkdir -p ${BASE_PATH}/prometheus/data
    sudo mkdir -p ${BASE_PATH}/guacd/data
    sudo chmod +x ${BASE_PATH}/proxy/smartcmp-proxy-agent

    create_service_config $BASE_PATH $CONTROLLER_ADDRESS $CLIENT_KEY
    config_guacd $BASE_PATH
    sudo chown -R gateway:gateway ${BASE_PATH}

}

function create_user() {
    sudo useradd gateway
}

function start_service() {
    SERVICE_LIST="consul prometheus guacd smartcmp-proxy-agent"
    systemctl daemon-reload
    for service_name in $SERVICE_LIST
    do
        sudo systemctl enable $service_name
        sudo systemctl start $service_name
        sudo systemctl status $service_name
    done
}

Help() {
    # Display Help
   echo "Usage: cloud_gateway_install [options]  VALUE"
   echo "Install CloudGateway"
   echo
   echo "Syntax: cloud_gateway_install [-c|h|k|p]"
   echo "options:"
   echo "-c, --controller    Controller url."
   echo "-h, --help          Print this Help."
   echo "-k, --key           Client key generated by SmartCMP."
   echo "-p, --path          User - defined installation path."
   echo
}

function main() {
    ADDRESS=$1
    KEY=$2
    USER_BASE_PAATH=$3
    create_user
    make_file $ADDRESS $KEY $USER_BASE_PATH
    start_service
    open_firewall_port
}

while true; do
    case "${1}" in
        -h|--help)
        shift;
        Help
        exit
        ;;
        -c|--controller)
        shift;
        if [[ -n "${1}" ]]; then
            echo -e "Url: value is ${1}"
            ADDRESS=${1}
            shift;
        fi
        ;;
        -k|--key)
        shift;
        if [[ -n "${1}" ]]; then
            echo -e "Key: value is ${1}"
            KEY=${1}
            shift;
        fi
        ;;
        -p|--path)
        shift;
        if [[ -n "${1}" ]]; then
            echo -e "Path: value is ${1}"
            USER_BASE_PATH=${1}
            shift;
        fi
        ;;
        --)
        shift;
        break;
        ;;
    esac
done

if  [ ! -n "$ADDRESS" ] ;then
        echo "Controller address is a required parameter, but it is not specified."
        exit 1
fi

if  [ ! -n "$KEY" ] ;then
        echo "Client key is a required parameter, but it is not specified."
        exit 1
fi

main $ADDRESS $KEY $USER_BASE_PATH
