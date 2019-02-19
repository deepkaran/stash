
ssh_key_path=~/ssh/deep_aws_us.pem
while read line; 
do 
    echo $line;
    ssh -n -i $ssh_key_path ubuntu@$line "df -h /data;"; 
done < data_ip.txt

