package services

// Фильтрация по программам держится на двух линуксовых вещах: списке .desktop
// и метках в nftables/cgroup. На других платформах этого кода нет вовсе.
const appRoutingSupported = true
