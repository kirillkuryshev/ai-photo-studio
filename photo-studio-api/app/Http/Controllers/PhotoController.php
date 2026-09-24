<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Storage; 
use Illuminate\Support\Str; 
use App\Models\UserHistory; 

class PhotoController extends Controller
{
    public function transform(Request $request)
    {

        if (!$request->hasFile('photo')) {
            return response()->json(['message' => 'Photo is required'], 400);
        }
        $request->validate(['photo' => 'image']);
        $photo = $request->file('photo');


        $pathBefore = $photo->store('user_histories', 'public');


        $historyRecord = UserHistory::create([
            'user_id' => $request->user()->id,
            'ai_model' => 'Restore Old Photos',
            'status' => 'in_progress',
            'image_before' => $pathBefore,
        ]);


        $response = Http::attach(
            'photo',
            $photo->get(),
            $photo->getClientOriginalName()
        )->post('http://127.0.0.1:8001/restore');

        if ($response->failed()) {
            $historyRecord->update(['status' => 'failed']); 
            return response()->json(['message' => 'AI service error'], 500);
        }


        $fileNameAfter = Str::random(40) . '.jpg';
        $pathAfter = 'user_histories/' . $fileNameAfter;
        

        Storage::disk('public')->put($pathAfter, $response->body());


        $historyRecord->update([
            'status' => 'completed',
            'image_after' => $pathAfter,
        ]);


        return response($response->body())
            ->header('Content-Type', $response->header('Content-Type'));
    }


    public function upscale4k(Request $request)
    {
        if (!$request->hasFile('photo')) {
            return response()->json(['message' => 'Photo is required'], 400);
        }
        $request->validate(['photo' => 'image']);
        $photo = $request->file('photo');

        $pathBefore = $photo->store('user_histories', 'public');

        $historyRecord = UserHistory::create([
            'user_id' => $request->user()->id,
            'ai_model' => 'Upscale Image to 4K', 
            'status' => 'in_progress',
            'image_before' => $pathBefore,
        ]);

        $response = Http::attach(
            'photo',
            $photo->get(),
            $photo->getClientOriginalName()
        )->post('http://127.0.0.1:8001/upscale-4k');

        if ($response->failed()) {
            $historyRecord->update(['status' => 'failed']); 
            return response()->json(['message' => 'AI service error'], 500);
        }

        $fileNameAfter = Str::random(40) . '.jpg';
        $pathAfter = 'user_histories/' . $fileNameAfter;
        
        Storage::disk('public')->put($pathAfter, $response->body());

        $historyRecord->update([
            'status' => 'completed',
            'image_after' => $pathAfter,
        ]);

        return response($response->body())
            ->header('Content-Type', $response->header('Content-Type'));
    }

    public function index(Request $request)
    {
        $userHistory = UserHistory::where(
            'user_id',
            $request->user()->id
        )->get();

        return response()->json([
            'data' => $userHistory
        ], 200);
    }
}