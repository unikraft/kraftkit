SOURCES = \
	gophers.txt \
	readme.txt \
	todo.txt \
	checklist.txt

ARCHIVES = \
	test_odc.cpio \
	test_svr4.cpio \
	test_svr4_crc.cpio

all: $(ARCHIVES)

test_odc.cpio: $(SOURCES)
	echo $(SOURCES) | tr " " "\n" | cpio -o --owner=0:0 --format=odc > $@

test_svr4.cpio: $(SOURCES)
	echo $(SOURCES) | tr " " "\n" | cpio -o --owner=0:0 --format=newc > $@

test_svr4_crc.cpio: $(SOURCES)
	echo $(SOURCES) | tr " " "\n" | cpio -o --owner=0:0 --format=crc > $@

clean:
	rm -f $(ARCHIVES) version.txt

.PHONY: all clean
