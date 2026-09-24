<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class UserHistory extends Model
{
    protected $table = 'user_history';

    protected $fillable = ['user_id', 'ai_model', 'status', 'image_before', 'image_after']; 
}
