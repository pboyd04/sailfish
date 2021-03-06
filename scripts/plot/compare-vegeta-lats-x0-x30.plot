set term svg
set output "vegeta-MEDIAN-LATENCY-0-30.svg"
set xlabel "Requests/Second Rate"
set ylabel "Median Latency"
set xrange [0:30]
plot \
    "odatalite-vegeta/LATENCIES-token.csv" using 1:3 title "odatalite" with lines,    \
    "sailfish-apache-vegeta/LATENCIES-token.csv" using 1:3 title "sailfish-apache" with lines,    \
    "sailfish-https-vegeta/LATENCIES-token.csv" using 1:3 title "sailfish" with lines,    \
    "sqlite-vegeta/LATENCIES-token.csv" using 1:3 title "sqlite" with lines, \
    "cim-vegeta/LATENCIES-token.csv" using 1:3 title "CIM" with lines
