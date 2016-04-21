## runlock: An etcd based single subprocess locker 

Lock a process to a single node using etcd. 

#### Installation:

Grab the right precompiled bin from github releases and put it in your path. Don't forget to `chmod +x` the bin.

OSX:

    curl -SL https://github.com/pnegahdar/runlock/releases/download/0.1.0/runlock_0.1.0_darwin_amd64.tar.gz \
        | tar -xzC /usr/local/bin --strip 1 && chmod +x /usr/local/bin/runlock
        
Nix:

    curl -SL https://github.com/pnegahdar/runlock/releases/download/0.1.0/runlock_0.1.0_linux_amd64.tar.gz \
        | tar -xzC /usr/local/bin --strip 1 && chmod +x /usr/local/bin/runlock
        
        
#### Usage:

    runlock run --etcd http://192.168.99.100:2379  --key test --ttl 20 --command "watch ls"
   
    NAME:
       runlock run - Run the locker.
    
    USAGE:
       runlock run [command options] [arguments...]
    
    OPTIONS:
       --etcd "http://localhost:2379"	The etcd enpoint to lock on [$RUNLOCK_ETCD_ENDPOINT]
       --key 				The key to use to lock the service. [$RUNLOCK_LOCK_KEY]
       --ttl "5"				The TTL in seconds for the lock [$RUNLOCK_TTL]
       --heartbeat "3"			How often to renew the lock [$RUNLOCK_HEARTBEAT]
       --command 				The command to run, like 'echo hello'    

