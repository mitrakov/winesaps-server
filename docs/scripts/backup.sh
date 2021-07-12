#!/bin/bash

# @mitrakov artem, 2017-07-25, winesaps.ru
# backup for 'rush' Database (nightly at 1:00 AM)
# cron cmd: 0 1 * * * /home/mitrakov/backup.sh
# please don't forget to create 2 files: .pwd_db and .pwd_host containing passwords for DB and remote host correspondingly,
# make them "chmod 400" to protect from other users
# also ensure there is sshpass installed (see www.tecmint.com/sshpass-non-interactive-ssh-login-shell-script-ssh-password)
# Be careful! At first run rsync may ask for RSA key fingerprint! So I recommend to run rsync for the first time in manual mode

PWD_DB=$(cat /home/mitrakov/.pwd_db)
PWD_HOST=$(cat /home/mitrakov/.pwd_host)
NOW=$(date +%Y-%m-%d_%H-%M-%S)

mkdir -p /home/mitrakov/backup
mysqldump -u tommy -p$PWD_DB rush > /home/mitrakov/backup/dump_$NOW.sql
/usr/local/bin/sshpass -p $PWD_HOST rsync -avzh --remove-source-files /home/mitrakov/backup/ mitrakov@winesaps.com:/home/mitrakov/backup/
