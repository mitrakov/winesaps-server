#include <time.h>           // time
#include <stdio.h>          // puts
#include <stdint.h>         // uint8_t
#include <pthread.h>        // pthread_create
#include <sys/fcntl.h>      // close

#ifdef __linux__
  #include <netdb.h>        // gethostbyname
  #include <unistd.h>       // usleep
  #include <stdlib.h>       // rand
  #include <string.h>       // memcpy
  #include <arpa/inet.h>    // inet_addr
  #include <sys/socket.h>   // socket
  #include <netinet/in.h>   // htons, sockaddr, sockaddr_in
  #define Sleep(x) usleep((x)*1000)
#elif defined __WIN32__
  #include <ws2tcpip.h>     // socklen_t
  #include <windows.h>      // socket
#endif

// constants
#define BUF_SIZ 256
#define ACK_SIZE 5
#define HEADER_SIZE 15
#define MIN(X, Y) (((X) < (Y)) ? (X) : (Y))

// forward declarations
void panic(const char* err);
void init_network();
void clean_console();
uint8_t next(uint8_t n);
void parse(uint8_t* msg, size_t len);
void* recv_handler(void* arg);

// statistics categories
char* categories[] = {
    "Time elapsed:      ",
	"RPS:               ",
    "Current used SIDs: ",
    "Current battles:   ",
    "Current users:     ",
    "Total battles:     ",
    "Total users:       ",
    "Senders count:     ",
    "Receivers count:   ",
    "Current AI count:  ",
    "Total AI spawned:  ",
    "Battle refs up:    ",
    "Battle refs down:  ",
    "Round refs up:     ",
    "Round refs down:   ",
    "Field refs up:     ",
    "Field refs down:   ",
    "Current env size:  ",
};
const int categoriesLen = sizeof(categories)/sizeof(char*);

// global variables (WTF?)
uint8_t fnCallMode = 0;
uint8_t dots = 0;       // to visualize that server is responding


/**
 * @brief winesaps statistics utility
 * @note on Windows add: LIBS += C:\Qt\MinGW\lib\libws2_32.a
 * @note linux cmd: gcc -Wall -g -O0 stat.c -lpthread -o statistics
 * @author mitrakov
 * @return 0
 */
int main(int argc, char* argv[]) {
    // check command line arguments
    if (argc < 2)
        {printf("Usage \"%s <host>\" or \"%s <host> <cmd>\"\n", argv[0], argv[0]); return 0;}
    fnCallMode = argc == 3; // call remote function on server (and then exit)

    // prepare server address
    init_network();
    struct hostent *host = gethostbyname(argv[1]);
    if (!host)
        panic("Cannot resolve address");
    struct sockaddr_in addr = {0};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(33996);
    memcpy(&addr.sin_addr, host->h_addr_list[0], host->h_length);

    // create socket
    int sock = socket(PF_INET, SOCK_DGRAM, 0);
    if (sock < 0)
        panic("Cannot create socket");

    // start listening thread
    pthread_t th;
    if (pthread_create(&th, NULL, recv_handler, (void*)(intptr_t)sock) != 0)
        panic("Cannot create thread");

    // prepare SwUDP parameters
    uint8_t id = 0;
    srand(time(NULL));
    uint32_t crcid = rand() << 16 | rand();

    // connect to the server
    char msg[] = {id, (crcid >> 24) & 0xFF, (crcid >> 16) & 0xFF, (crcid >> 8) & 0xFF, crcid & 0xFF, 0xFD};
    if (sendto(sock, msg, sizeof(msg), 0, (struct sockaddr*) &addr, sizeof(struct sockaddr_in)) < 0)
        panic("Send socket error!");

    // main loop
    puts("Waiting for server...");
    while (1) {
        Sleep(3000);
        id = next(id);
        char msg[BUF_SIZ] = {id, (crcid >> 24) & 0xFF, (crcid >> 16) & 0xFF, (crcid >> 8) & 0xFF, crcid & 0xFF, 0, 0,
                   0x21, 0x39, 0xFF, 0xB2, 0, 0, 1, fnCallMode ? 0xF1 : 0xF0};
        size_t msgLen = HEADER_SIZE;
        if (fnCallMode) {
            size_t n = MIN(strlen(argv[2]), BUF_SIZ - msgLen);
            memcpy(msg + msgLen, argv[2], n);
            msg[13] += n;
            msgLen += n;
        }
        if (sendto(sock, msg, msgLen, 0, (struct sockaddr*) &addr, sizeof(addr)) < 0)
            panic("Send socket error");
        if (fnCallMode) break;
    }

    // finishing
    getchar();
    close(sock);
    return EXIT_SUCCESS;
}



/**
 * @brief pthread handler that receives incoming messages
 * @param arg - socket converted to void*
 * @return NULL
 */
void* recv_handler(void* arg) {
    int sock = (int)((uintptr_t)arg);
    while (1) {
        uint8_t buffer[BUF_SIZ] = {0};
        struct sockaddr addr = {0};
        socklen_t addrlen = sizeof(addr);
        int len = recvfrom(sock, (char*)buffer, sizeof(buffer), 0, &addr, &addrlen);
        if (len > ACK_SIZE) {
            if (sendto(sock, (char*)buffer, 5, 0, (struct sockaddr*) &addr, addrlen) < 0)
                panic("Send socket error!");
            parse(buffer, len);
        } else if (len == 0) {
            puts("Disconnected!");
            break;
        } else if (len < 0)
            panic("Receive socket error");
    }
    return NULL;
}



/**
 * @brief parses incoming message
 * @param msg - bytearray
 * @param len - size of bytearray
 */
void parse(uint8_t* msg, size_t len) {
    size_t i;
    if (len > HEADER_SIZE) {
        uint8_t err = msg[HEADER_SIZE];
        if (!err && !fnCallMode) {
            clean_console();
            puts("== WINESAPS STATISTICS ==");
            puts(++dots % 4 == 3 ? "..." : dots % 4 == 2 ? ".." : dots % 4 == 1 ? "." : "");
            for (i = HEADER_SIZE+1; i+2 < len; i+=3) {
                uint8_t category = msg[i];
                uint8_t valueH = msg[i+1];
                uint8_t valueL = msg[i+2];
                uint16_t value = valueH*256 + valueL;

                if (category < categoriesLen)
                    printf("%s %5u\n", categories[category], value);
                else printf("Unknown parameter: %5u\n", value);
            }
        } else printf("Response code (%d)\n", err);
    }
}



/**
 * @param n - current ID
 * @return next non-zero ID for SwUDP protocol
 */
uint8_t next(uint8_t n) {
    uint8_t result = n+1;
    uint8_t ok = result != 0 && result != 1;
    return ok ? result : next(result);
}



/**
 * @brief crossplatform panic function that prints a message and terminates the program
 * @param err - user defined error message
 */
void panic(const char* err) {
#ifdef __linux__
    perror(err);
    puts("");
    exit(1);
#elif defined __WIN32__
    char buf[BUF_SIZ] = {0};
    FormatMessageA(FORMAT_MESSAGE_FROM_SYSTEM, NULL, GetLastError(),
                  MAKELANGID(LANG_NEUTRAL, SUBLANG_DEFAULT), buf, sizeof(buf), NULL);
    puts(err);
    puts(buf);
    exit(1);
#endif
}



/**
 * @brief crossplatform function to initialize the networking
 * @note on Linux does nothing
 */
void init_network() {
#ifdef __WIN32__
    WSADATA data;
    if (WSAStartup(MAKEWORD(2, 2), &data) != 0)
        panic("Cannot startup WS2");
#endif
}



/**
 * @brief crossplatform function to clear the console
 */
void clean_console() {
#ifdef __linux__
    system("clear");
#elif defined __WIN32__
    system("cls");
#endif
}
