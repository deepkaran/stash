
ssh_key_path=~/ssh/deep_aws_us.pem
rpm_path=~/Downloads/couchbase-server-enterprise_vulcan-ubuntu16.04_amd64.deb
rpm_name=couchbase-server-enterprise_vulcan-ubuntu16.04_amd64.deb

##create FS on /dev/xvdb, mount on /data
while read line; 
do 
    ssh -n -i $ssh_key_path ubuntu@$line "sudo mkfs -t ext4 /dev/xvdb; sudo mkdir /data; sudo mount /dev/xvdb /data;"; 
    ssh -n -i $ssh_key_path ubuntu@$line "df -h /data;"; 
done < data_ip.txt
#
###setup THP, swappiness, download CB, python, install CB
while read line; 
do 
    scp -i $ssh_key_path $rpm_path ubuntu@$line:/home/ubuntu; 
    ssh -n -i $ssh_key_path ubuntu@$line "exec sudo su -c 'echo never > /sys/kernel/mm/transparent_hugepage/enabled'"; 
    ssh -n -i $ssh_key_path ubuntu@$line "exec sudo su -c 'echo never > /sys/kernel/mm/transparent_hugepage/defrag'"; 
    ssh -n -i $ssh_key_path ubuntu@$line "sudo sysctl vm.swappiness=0"; 
    ssh -n -i $ssh_key_path ubuntu@$line "sudo apt-get -qq update; sudo apt-get --yes install python; sudo apt-get --yes install python-httplib2"; 
    ssh -n -i $ssh_key_path ubuntu@$line "sudo dpkg -i $rpm_name"; 
    ssh -n -i $ssh_key_path ubuntu@$line "sudo chown couchbase /data; sudo chgrp couchbase /data; sudo chmod 777 /data"; 
done < data_ip.txt

#while read line; 
#do 
#    ssh -n -i $ssh_key_path ubuntu@$line "sudo mkfs -t ext4 /dev/xvdb; sudo mkdir /data; sudo mount /dev/xvdb /data;"; 
#    ssh -n -i $ssh_key_path ubuntu@$line "df -h /data;"; 
#done < index_ip.txt
#
###setup THP, swappiness, download CB, python, install CB
#while read line; 
#do 
#    scp -i $ssh_key_path $rpm_path ubuntu@$line:/home/ubuntu; 
#    ssh -n -i $ssh_key_path ubuntu@$line "exec sudo su -c 'echo never > /sys/kernel/mm/transparent_hugepage/enabled'"; 
#    ssh -n -i $ssh_key_path ubuntu@$line "exec sudo su -c 'echo never > /sys/kernel/mm/transparent_hugepage/defrag'"; 
#    ssh -n -i $ssh_key_path ubuntu@$line "sudo sysctl vm.swappiness=0"; 
#    ssh -n -i $ssh_key_path ubuntu@$line "sudo apt-get -qq update; sudo apt-get --yes install python; sudo apt-get --yes install python-httplib2"; 
#    ssh -n -i $ssh_key_path ubuntu@$line "sudo dpkg -i $rpm_name"; 
#    ssh -n -i $ssh_key_path ubuntu@$line "sudo chown couchbase /data; sudo chgrp couchbase /data; sudo chmod 777 /data"; 
#done < index_ip.txt
#
###setup node
#
kv=ec2-18-233-171-111.compute-1.amazonaws.com
#
#while read line; 
#do 
#    ssh -n -i $ssh_key_path ubuntu@$line "sudo /opt/couchbase/bin/couchbase-cli node-init -u Administrator -p password -c $line --node-init-hostname $line --node-init-data-path /data --node-init-index-path /data"; 
#    ssh -n -i $ssh_key_path ubuntu@$line "sudo /opt/couchbase/bin/couchbase-cli server-add -c $kv:8091 --username Administrator --password password --server-add $line:8091 --server-add-username Administrator --server-add-password password --services data,query"; 
#done < data_ip.txt
#
#
#while read line; 
#do 
#    ssh -n -i $ssh_key_path ubuntu@$line "sudo /opt/couchbase/bin/couchbase-cli node-init -u Administrator -p password -c $line --node-init-hostname $line --node-init-data-path /data --node-init-index-path /data"; 
#    ssh -n -i $ssh_key_path ubuntu@$line "sudo /opt/couchbase/bin/couchbase-cli server-add -c $kv:8091 --username Administrator --password password --server-add $line:8091 --server-add-username Administrator --server-add-password password --services index"; 
#done < index_ip.txt


#sudo apt-get install python-pip
#sudo apt-get -y install python3-pip
#sudo pip3 install pandas
#sudo pip3 install matplotlib

#python3 gsistats.py -r 1 -u Administrator -p password -i "ec2-18-212-82-243.compute-1.amazonaws.com"
#python3 gsistats.py -r 4 -u Administrator -p password -i "ec2-18-212-82-243.compute-1.amazonaws.com"
