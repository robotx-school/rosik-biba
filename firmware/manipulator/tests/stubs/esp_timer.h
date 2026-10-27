#pragma once
using esp_timer_handle_t=void*;
struct esp_timer_create_args_t {void(*callback)(void*);const char *name;};
static int esp_timer_create(esp_timer_create_args_t*,esp_timer_handle_t*){return 0;}
static int esp_timer_start_periodic(esp_timer_handle_t,int){return 0;}
#define ESP_ERROR_CHECK(value) ((void)(value))
