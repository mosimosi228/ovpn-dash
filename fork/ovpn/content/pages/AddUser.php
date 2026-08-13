<div class="container">
    <div class="card card-container">
        <div align='center'><h3>Add User</h3></div>
        <br />
        <?php
        $users = [];
        if (file_exists('./users.json')) {
            $users = json_decode(file_get_contents('./users.json'), true);
        }

        $user = $_POST['user'];
        $pass = $_POST['password'];

        if (!empty($user) && !empty($pass)) {
            $users[$user] = [
                'password' => md5($pass)
            ];
            file_put_contents('./users.json', json_encode($users, JSON_PRETTY_PRINT));
            echo '<div class="well" align=\'center\' style=\'color: green;\'>User added successfully!</div>';
        } elseif(isset($_POST['user'])) {
            echo '<div class="well" align=\'center\' style=\'color: red;\'>Failed to add User!</div>';
        }
        ?>
        <form class="form-signin" action="index.php?section=add_user" method='POST'>
            <span id="reauth-email" class="reauth-email"></span>
            <input type="text" id="inputUserName" class="form-control" name='user' placeholder="Name" required autofocus>
            <input type="password" id="inputUserPassword" class="form-control" name='password' placeholder="Password" required>
            <button class="btn btn-lg btn-primary btn-block btn-signin" type="submit">Add</button>
        </form><!-- /form -->
    </div><!-- /card-container -->
</div><!-- /container -->
