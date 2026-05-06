# Redis Firewall Apply Procedure

## Когда применять

При первом deploy на новый VM, и после любого `iptables -F` (например, после рестарта VM, если `iptables-persistent` не настроен).

## Как применять

1. SSH под пользователем с sudo:
   `ssh user@<host>`
2. Выполнить скрипт:
   `sudo bash /opt/sms/scripts/redis-firewall.sh`
3. Verify:
   `sudo iptables -L INPUT -n | grep ':6379'` — должны быть DROP-правила.
4. Probe извне:
   `nc -zv <host> 6379 -w 3` с локального хоста — должен timeout.

## Persistence

iptables правила НЕ переживают reboot без `iptables-persistent`. После применения:
`sudo apt-get install -y iptables-persistent && sudo netfilter-persistent save`

## Rollback

Снять только правила Redis (НЕ `-F INPUT` — он флашит всю цепочку, ломает SSH-rules и пр.):

```
sudo iptables -D INPUT -p tcp --dport 6379 -j DROP
sudo iptables -D INPUT -i lo -p tcp --dport 6379 -j ACCEPT
sudo iptables -D INPUT -p tcp --dport 6379 -m state --state ESTABLISHED,RELATED -j ACCEPT
# Docker bridge ACCEPT: bridge name через `ip -br link | grep -E 'docker0|br-'`, удалить аналогично.
sudo netfilter-persistent save
```

Используется только при инциденте, когда firewall блокирует легитимный traffic. После rollback Redis открыт всему миру — закрыть как можно скорее.
