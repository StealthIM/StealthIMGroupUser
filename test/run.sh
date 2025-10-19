#!/bin/bash
mkdir -p ./test_cache

mkdir -p ./test_cache/db
mkdir -p ./test_cache/session
mkdir -p ./test_cache/user
mkdir -p ./test_cache/groupuser

NOWPWD=$(pwd)

echo "PWD: ${NOWPWD}"

cat <<EOF > "${NOWPWD}/test_cache/db/config.toml"
[mysql]
host = "127.0.0.1"           # Mysql地址
port = 3306                  # Mysql端口
maxconn = 50                 # 最大连接数
minconn = 10                 # 最小连接数
user = "root"                # 用户名，首次建议使用root，之后可以创建新用户
password = "wMTs5aXwfjndimtT"  # 密码

[redis]
host = "127.0.0.1"           # Redis地址
port = 6379                  # Redis端口
dbname = 0                   # 数据库编号，通常无需改动
password = "wMTs5aXwfjndimtT"  # 密码
cachetime = 3600             # 缓存时间，单位秒

[grpc]
host = "127.0.0.1" # GRPC地址
port = 50051       # GRPC监听端口
log = true         # 启用日志，调试功能，上线建议关闭
EOF

cat <<EOF > "${NOWPWD}/test_cache/session/config.toml"
[grpc]
host = "127.0.0.1" # GRPC地址
port = 50054       # GRPC监听端口
log = true         # 启用日志，调试功能，上线建议关闭

[dbgateway]
host = "127.0.0.1"
port = 50051
conn_num = 5
sql_timeout = 5000 # 单位：ms

[cache]
mem_timeout = 60    # 单位 s
mem_maxsize = 100   # 单位 KB
mem_cleantime = 360 # 单位 s

[session]
expire_hours = 24   # 会话有效期（小时）
clean_interval = 60 # 清理间隔（分钟）
EOF

cat <<EOF > "${NOWPWD}/test_cache/user/config.toml"
[grpc]
host = "127.0.0.1" # GRPC地址
port = 50055       # GRPC监听端口
log = true         # 启用日志，调试功能，上线建议关闭

[dbgateway]
host = "127.0.0.1"
port = 50051
conn_num = 5
sql_timeout = 5000 # 单位：ms

[session]
host = "127.0.0.1"
port = 50054
conn_num = 5
EOF

cat <<EOF > "${NOWPWD}/test_cache/groupuser/config.toml"
[grpc]
host = "127.0.0.1" # GRPC地址
port = 50058       # GRPC监听端口
log = false        # 启用日志，调试功能，上线建议关闭

[dbgateway]
host = "127.0.0.1"
port = 50051
conn_num = 5
sql_timeout = 5000 # 单位：ms

[user]
host = "127.0.0.1"
port = 50055
conn_num = 5
sql_timeout = 5000 # 单位：ms

[security]
password_salt = "<stim_you_salt>"
EOF

wget https://github.com/StealthIM/StealthIMDB/releases/latest/download/StealthIMDB -O ./test_cache/db/StealthIMDB
chmod +x ./test_cache/db/StealthIMDB
wget https://github.com/StealthIM/StealthIMSession/releases/latest/download/StealthIMSession -O ./test_cache/session/StealthIMSession
chmod +x ./test_cache/session/StealthIMSession
wget https://github.com/StealthIM/StealthIMUser/releases/latest/download/StealthIMUser -O ./test_cache/user/StealthIMUser
chmod +x ./test_cache/user/StealthIMUser

cp ../bin/StealthIMGroupUser ./test_cache/groupuser/StealthIMGroupUser
chmod +x ./test_cache/groupuser/StealthIMGroupUser

echo "Start DB"
cd ${NOWPWD}/test_cache/db && ./StealthIMDB --config=${NOWPWD}/test_cache/db/config.toml > ${NOWPWD}/test_cache/db.log 2>&1 &

sleep 3s

echo "Start Session"
cd ${NOWPWD}/test_cache/session && ./StealthIMSession --config=${NOWPWD}/test_cache/session/config.toml > ${NOWPWD}/test_cache/session.log 2>&1 &

sleep 3s

echo "Start User"
cd ${NOWPWD}/test_cache/user && ./StealthIMUser --config=${NOWPWD}/test_cache/user/config.toml > ${NOWPWD}/test_cache/user.log 2>&1 &

sleep 3s

echo "Start GroupUser"
cd ${NOWPWD}/test_cache/groupuser && ./StealthIMGroupUser --config=${NOWPWD}/test_cache/groupuser/config.toml > ${NOWPWD}/test_cache/groupuser.log 2>&1 &

sleep 15s
echo "Start Test"
echo "::group::Test Log"
(pytest test_group_user.py -v; echo "$?">${NOWPWD}/test_cache/.ret) | tee ${NOWPWD}/test_cache/test.log
RETVAL=$(cat ${NOWPWD}/test_cache/.ret)
echo "::endgroup::"

sleep 3s
echo "Clean"
ps -aux | grep '[S]tealthIM' | awk '{print $2}' | xargs kill -9

sleep 2s
echo "::group::DB Log"
cat ${NOWPWD}/test_cache/db.log
echo "::endgroup::"

echo "::group::Session Log"
cat ${NOWPWD}/test_cache/session.log
echo "::endgroup::"

echo "::group::User Log"
cat ${NOWPWD}/test_cache/user.log
echo "::endgroup::"

echo "::group::GroupUser Log"
cat ${NOWPWD}/test_cache/groupuser.log
echo "::endgroup::"

if [ "$RETVAL" -ne 0 ]; then
    echo "::error title=Test failed::Test Log: ${NOWPWD}/test_cache/test.log"
    while IFS= read -r line
    do
        echo "::error::$line"
    done < "${NOWPWD}/test_cache/test.log"
fi

rm ${NOWPWD}/test_cache -r

exit ${RETVAL}
