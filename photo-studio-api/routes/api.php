<?php

use Illuminate\Http\Request;
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\AuthController;
use App\Http\Controllers\PhotoController;

Route::post('/login', [AuthController::class, 'login']);
Route::post('/signup', [AuthController::class, 'signup']);

Route::middleware('auth:sanctum')->group(function () {
    Route::post('/logout', [AuthController::class, 'logout']);

    Route::post('/photos/transform', [PhotoController::class, 'transform']);
    Route::post('/photos/upscale-4k', [PhotoController::class, 'upscale4k']);
    Route::post('/photos/colorize', [PhotoController::class, 'colorize']);

    Route::get('/user-history', [PhotoController::class, 'index']);
});