## runlock: An etcd based single subprocess locker 

Lock a process to a single node using etcd. 

#### Installation:

Grab the right precompiled bin from github releases and put it in your path. Don't forget to `chmod +x` the bin.

OSX:

    curl -SL https://github.com/pnegahdar/runlock/releases/download/0.1.2/runlock_0.1.2_darwin_amd64.tar.gz \
        | tar -xzC /usr/local/bin --strip 1 && chmod +x /usr/local/bin/runlock
        
Nix:

    curl -SL https://github.com/pnegahdar/runlock/releases/download/0.1.2/runlock_0.1.2_linux_amd64.tar.gz \
        | tar -xzC /usr/local/bin --strip 1 && chmod +x /usr/local/bin/runlock
        
        
#### Usage:

    runlock run --etcd http://192.168.99.100:2379  --key test --value NODE1 --ttl 20 --on-acquire "echo locked" --on-release "echo lost"
   
    NAME:
       main run - Run the locker.
    
    USAGE:
       main run [command options] [arguments...]
    
    OPTIONS:
       --etcd "http://localhost:2379"	The etcd enpoint to lock on [$RUNLOCK_ETCD_ENDPOINT]
       --key 				The key to use to lock the service. [$RUNLOCK_LOCK_KEY]
       --ttl "5"				The TTL in seconds for the lock [$RUNLOCK_TTL]
       --heartbeat "3"			How often to renew the lock [$RUNLOCK_HEARTBEAT]
       --on-acquire 			The command to run when lock is acquired.
       --on-release 			The command to run when lock is released.
       --value 				The identifier of this node for acquiring the lock
   
