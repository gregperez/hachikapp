module example.com/absolute-replacement

go 1.26.6

require example.com/original v1.0.0

replace example.com/original => /tmp/authBase
