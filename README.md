# Elasticsearch Fitter
Deletes old indices to fit Elasticsearch data in disk space


### `-version`


### `-skip`
Index won't be removed

    -skip=".kibana" -skip="logstash-2006-01-02"


### `-space=15`
% of disk space to be freed


### `-duration=1h`
Check frequency
https://golang.org/pkg/time/#ParseDuration


### `-server="http://127.0.0.1:9200"`
ElasticSearch URL