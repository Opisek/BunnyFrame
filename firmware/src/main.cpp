#include "EPD_7in3e.h"

#include <WiFi.h>
#include <HTTPClient.h>

#include "esp_timer.h"

const char* ssid = "";
const char* password = "";

int statusCode = 0;
int ledPosition = 3;
int ledState = 0;
int ledTick = 0;

void IRAM_ATTR onTimer() {
    ledTick++;
    if (ledPosition == 3) {
        ledState = 0;
        if (ledTick == 5) {
            ledTick = 0;
            ledPosition = 0;
        }
    } else {
        int currentBit = (statusCode >> (2-ledPosition)) & 1;
        int onTicks = 1 + 2 * currentBit;

        ledState = ledTick <= onTicks;
        if (ledTick > onTicks) {
            ledTick = 0;
            ledPosition++;
        }
    }

    digitalWrite(15, ledState);
}

hw_timer_t *timer = NULL;

// horizontal:
//int width = 800;
//int height = 480;
//byte* url = "https://bunny.opisek.net/?width=800&height=480&colors=000000,ffffff,e6e600,cc0000,0033cc,00cc00";

// vertical:
int width = 480;
int height = 800;
const char* url = "https://bunny.opisek.net/?width=480&height=800&colors=000000,ffffff,e6e600,cc0000,0033cc,00cc00";

void setup() {
    pinMode(15, OUTPUT); 
    digitalWrite(15, LOW);
    timer = timerBegin(0, 80, true);
    timerAttachInterrupt(timer, &onTimer, true);
    timerAlarmWrite(timer, 100000, true);
    timerAlarmEnable(timer);

    statusCode = 0;

    WiFi.begin(ssid, password);

    while (WiFi.status() != WL_CONNECTED) delay(100);

    statusCode = 1;

    HTTPClient http;
    http.begin(url);
    int httpCode = http.GET();

    UBYTE* img = (UBYTE*)ps_malloc(width / 2 * height);
    int rowBufLen = width * 3;
    //rowBufLen += (0b100 - (rowBufLen % 0b100)) & 0b100;
    uint8_t* rowBuf = (uint8_t*)malloc(rowBufLen);

    if (httpCode == HTTP_CODE_OK) {
        statusCode = 2;
        Serial.print("downloading");
        WiFiClient* stream = http.getStreamPtr();
        bool readingHeader = true;
        int y = 0;
        while (http.connected() && stream->available()) {
            if (readingHeader) {
                int len = stream->readBytes(rowBuf, 0x36);
                readingHeader = false;
            } else if (y < height) {
                int len = stream->readBytes(rowBuf, rowBufLen);
                for (int x = 0; x < width; x++) {
                    int pixelColor = rowBuf[x * 3] + (rowBuf[x * 3 + 1] << 8) + (rowBuf[x * 3 + 2] << 16);
                    UBYTE einkPixelColor;
                    switch (pixelColor) {
                        case 0x000000:
                            einkPixelColor = EPD_7IN3E_BLACK;
                        break;
                        case 0xe6e600:
                            einkPixelColor = EPD_7IN3E_YELLOW;
                        break;
                        case 0xcc0000:
                            einkPixelColor = EPD_7IN3E_RED;
                        break;
                        case 0x0033cc:
                            einkPixelColor = EPD_7IN3E_BLUE;
                        break;
                        case 0x00cc00:
                            einkPixelColor = EPD_7IN3E_GREEN;
                        break;
                        case 0xffffff:
                        default:
                            einkPixelColor = EPD_7IN3E_WHITE;
                        break;
                    }

                    // horizontal:
                    //int bufLocation = (height - 1 - y) * (width / 2) + (int)(i/2);
                    //UBYTE curPixel = img[bufLocation];
                    //if (x % 2 == 0) curPixel = (einkPixelColor << 4) | (curPixel & 0x0F);
                    //else curPixel = (curPixel & 0xF0) | einkPixelColor;
                    //img[bufLocation] = curPixel;

                    // vertical:
                    int bufLocation = x * (height / 2) + (int)(y/2);
                    UBYTE curPixel = img[bufLocation];
                    if (y % 2 == 0) curPixel = (einkPixelColor << 4) | (curPixel & 0x0F);
                    else curPixel = (curPixel & 0xF0) | einkPixelColor;
                    img[bufLocation] = curPixel;
                }
                y++;
            } else {
                int len = stream->readBytes(rowBuf, rowBufLen);
            }
        }
        statusCode = 3;
        DEV_Module_Init();
        statusCode = 4;
        EPD_7IN3E_Init();
        statusCode = 5;
        EPD_7IN3E_Display(img);
        EPD_7IN3E_Sleep();
        statusCode = 6;
    } else {
        statusCode = 7;
        Serial.print("download fail");
    }
    http.end();
}

void loop() {

}