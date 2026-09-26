module example.com/remote-replacement

go 1.26.6

require example.com/original v1.0.0

replace example.com/original v1.0.0 => example.com/fork v1.0.1
