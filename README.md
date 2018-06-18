# Elasticsearch Fitter
Deletes old indices to fit Elasticsearch data in disk space


### `-version`


### `-skip`
Index patterns won't be removed

    -skip="\.kibana" -skip="logstash-\d\d\d\d-\d\d-\d\d"


### `-space=15`
% of disk space to be freed


### `-duration=1h`
Check frequency
https://golang.org/pkg/time/#ParseDuration


### `-server="http://$(hostname --ip-address):9200"`
ElasticSearch URL