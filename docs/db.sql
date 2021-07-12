-- --------------------------------------------------------
-- Host:                         winesaps.ru
-- Server version:               5.7.19 - MySQL Community Server (GPL)
-- Server OS:                    Linux
-- HeidiSQL Version:             9.3.0.4984
-- --------------------------------------------------------

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET NAMES utf8mb4 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;

-- Dumping database structure for rush
DROP DATABASE IF EXISTS `rush`;
CREATE DATABASE IF NOT EXISTS `rush` /*!40100 DEFAULT CHARACTER SET utf8 */;
USE `rush`;


-- Dumping structure for table rush.ability
DROP TABLE IF EXISTS `ability`;
CREATE TABLE IF NOT EXISTS `ability` (
  `ability_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `name` enum('Snorkel','ClimbingShoes','SouthWester','VoodooMask','SapperShoes','Sunglasses','7','8','9','10','11','12','13','14','15','16','17','SpPack2','19','20','21','22','23','24','25','26','27','28','29','30','31','32','Miner','Builder','Shaman','Grenadier','TeleportMan') NOT NULL DEFAULT 'Snorkel' COMMENT 'ability name',
  `days` tinyint(3) unsigned NOT NULL DEFAULT '1' COMMENT 'days to expire',
  `gems` int(10) unsigned NOT NULL DEFAULT '1' COMMENT 'cost',
  PRIMARY KEY (`ability_id`),
  UNIQUE KEY `product` (`name`,`days`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='all possible abilities and its prices';

-- Data exporting:
INSERT INTO `ability` VALUES (1,'Snorkel',1,13),(2,'Snorkel',3,36),(3,'Snorkel',7,66),(4,'ClimbingShoes',1,12),(5,'ClimbingShoes',3,32),(6,'ClimbingShoes',7,60),(7,'SouthWester',1,13),(8,'SouthWester',3,34),(9,'SouthWester',7,64),(10,'VoodooMask',1,16),(11,'VoodooMask',3,43),(12,'VoodooMask',7,80),(13,'SapperShoes',1,11),(14,'SapperShoes',3,30),(15,'SapperShoes',7,56),(16,'Sunglasses',1,8),(17,'Sunglasses',3,23),(18,'Sunglasses',7,42),(19,'Miner',1,10),(20,'Miner',3,27),(21,'Miner',7,50),(22,'Builder',1,10),(23,'Builder',3,26),(24,'Builder',7,48),(25,'Shaman',1,10),(26,'Shaman',3,28),(27,'Shaman',7,52),(28,'Grenadier',1,8),(29,'Grenadier',3,22),(30,'Grenadier',7,40),(31,'TeleportMan',1,14),(32,'TeleportMan',3,39),(33,'TeleportMan',7,72),(34,'SpPack2',255,120);


-- Dumping structure for table rush.friend
DROP TABLE IF EXISTS `friend`;
CREATE TABLE IF NOT EXISTS `friend` (
  `friend_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `user_id` bigint(20) unsigned NOT NULL COMMENT 'reference to a user',
  `friend_user_id` bigint(20) unsigned NOT NULL COMMENT 'reference to a friend',
  PRIMARY KEY (`friend_id`),
  UNIQUE KEY `user_id` (`user_id`,`friend_user_id`),
  KEY `friend_user_id` (`friend_user_id`),
  CONSTRAINT `fkuser_id` FOREIGN KEY (`user_id`) REFERENCES `user` (`user_id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `friend_user_id` FOREIGN KEY (`friend_user_id`) REFERENCES `user` (`user_id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='list of friends';

-- Data exporting was unselected.


-- Dumping structure for table rush.payment
DROP TABLE IF EXISTS `payment`;
CREATE TABLE IF NOT EXISTS `payment` (
  `payment_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `user_id` bigint(20) unsigned NOT NULL COMMENT 'reference to a user',
  `order_id` varchar(64) NOT NULL COMMENT 'order_id',
  `sku` enum('gems_pack_small','gems_pack','gems_pack_big') NOT NULL DEFAULT 'gems_pack' COMMENT 'SKU',
  `stamp` timestamp NULL DEFAULT NULL COMMENT 'purchase date (make it nullable to avoid bugs)',
  `data` varchar(200) NOT NULL DEFAULT '' COMMENT 'service info, e.g. token',
  `state` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT 'status (0 (purchased), 1 (canceled), or 2 (refunded))',
  `checked` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT 'signature checked',
  `gems` int(10) unsigned NOT NULL DEFAULT '0' COMMENT 'total gems added',
  PRIMARY KEY (`payment_id`),
  UNIQUE KEY `order_id` (`order_id`),
  KEY `payment_user` (`user_id`),
  CONSTRAINT `payment_user` FOREIGN KEY (`user_id`) REFERENCES `user` (`user_id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='payment transactions';

-- Data exporting was unselected.


-- Dumping structure for table rush.promocode
DROP TABLE IF EXISTS `promocode`;
CREATE TABLE IF NOT EXISTS `promocode` (
  `promocode_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `user_id` bigint(20) unsigned NOT NULL COMMENT 'new user id',
  `inviter_user_id` bigint(20) unsigned NOT NULL COMMENT 'inviter user id',
  `promo` enum('Pending','Used') NOT NULL DEFAULT 'Pending' COMMENT 'whether promo code used or not',
  PRIMARY KEY (`promocode_id`),
  UNIQUE KEY `friend_user` (`user_id`,`inviter_user_id`),
  KEY `fk_inviterid` (`inviter_user_id`),
  CONSTRAINT `fk_inviterid` FOREIGN KEY (`inviter_user_id`) REFERENCES `user` (`user_id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_userid` FOREIGN KEY (`user_id`) REFERENCES `user` (`user_id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='Table for users signed up with a promocode';

-- Data exporting was unselected.


-- Dumping structure for table rush.rating
DROP TABLE IF EXISTS `rating`;
CREATE TABLE IF NOT EXISTS `rating` (
  `rating_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `user_id` bigint(20) unsigned NOT NULL COMMENT 'reference to a user',
  `type` enum('General','Weekly') NOT NULL DEFAULT 'General' COMMENT 'rating type (general, weekly, etc.)',
  `wins` int(10) unsigned NOT NULL DEFAULT '0' COMMENT 'victory count',
  `losses` int(10) unsigned NOT NULL DEFAULT '0' COMMENT 'defeats count',
  `victory_diff` int(11) GENERATED ALWAYS AS ((cast(`wins` as signed) - cast(`losses` as signed))) STORED NOT NULL COMMENT '(virtual field) difference: victories - defeats',
  `score_diff` int(11) NOT NULL DEFAULT '0' COMMENT 'total score difference',
  PRIMARY KEY (`rating_id`),
  UNIQUE KEY `position` (`user_id`,`type`),
  KEY `victory_diff` (`victory_diff`),
  KEY `score_diff` (`score_diff`),
  CONSTRAINT `fk_user_id` FOREIGN KEY (`user_id`) REFERENCES `user` (`user_id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='full rating (attention! field victory_diff is GENERATED)';

-- Data exporting was unselected.


-- Dumping structure for function rush.sp_buy
DROP FUNCTION IF EXISTS `sp_buy`;
DELIMITER //
CREATE FUNCTION `sp_buy`(`user_id` BIGINT UNSIGNED, `code` TINYINT UNSIGNED, `days` TINYINT UNSIGNED) RETURNS int(10) unsigned
    READS SQL DATA
    COMMENT 'procedure to buy abilities for gems'
BEGIN
    DECLARE myGems, cost, ok, t INT UNSIGNED DEFAULT 0;
    SELECT gems INTO myGems FROM user WHERE user.user_id = user_id;
    SELECT gems, count(ability_id) INTO cost, ok FROM ability WHERE ability.name = code AND ability.days = days;
    IF ok = 1 THEN
      IF myGems >= cost THEN
        SET t = IF(days != 0xFF, days, 3652); -- 0xFF means 10 years (since 2018-05-02)
        INSERT INTO user_ability (user_id, name, expire) VALUES (user_id, code, DATE_ADD(CURRENT_TIMESTAMP, INTERVAL t DAY)) ON DUPLICATE KEY UPDATE expire = DATE_ADD(expire, INTERVAL t DAY);
        UPDATE user SET gems = gems - cost WHERE user.user_id = user_id;
		  RETURN cost;
      ELSE
        SIGNAL SQLSTATE '12345' SET MESSAGE_TEXT = 'Insufficient gems';
      END IF;
    ELSE
      SIGNAL SQLSTATE '12345' SET MESSAGE_TEXT = 'Incorrect product';
    END IF;
END//
DELIMITER ;


-- Dumping structure for procedure rush.sp_friend
DROP PROCEDURE IF EXISTS `sp_friend`;
DELIMITER //
CREATE PROCEDURE `sp_friend`(IN `user_id` BIGINT UNSIGNED, IN `friend_user_id` BIGINT UNSIGNED)
    NO SQL
    DETERMINISTIC
    COMMENT 'procedure to check friend lists'
BEGIN
IF (user_id = friend_user_id) THEN
  SIGNAL SQLSTATE '12345' SET MESSAGE_TEXT = 'You cannot be friends with yourself';
END IF;
END//
DELIMITER ;


-- Dumping structure for procedure rush.sp_user
DROP PROCEDURE IF EXISTS `sp_user`;
DELIMITER //
CREATE PROCEDURE `sp_user`(IN `name` VARCHAR(64), IN `email` VARCHAR(64))
    NO SQL
    DETERMINISTIC
    COMMENT 'procedure to check user data accuracy'
BEGIN
IF (name  REGEXP '^[a-zA-Z0-9_\\+\\-\\$\\.]{4,}$') = 0 THEN
  SIGNAL SQLSTATE '12345' SET MESSAGE_TEXT = 'Incorrect name length or format';
END IF;
-- IF (email REGEXP '^[a-zA-Z0-9_\\+\\-\\$\\.]{1,30}@[a-zA-Z0-9_\\+\\-\\$\\.]{1,30}\\.[a-zA-Z]{1,4}$') = 0 THEN
  -- SIGNAL SQLSTATE '12345' SET MESSAGE_TEXT = 'Incorrect email length or format';
-- END IF;
END//
DELIMITER ;


-- Dumping structure for table rush.user
DROP TABLE IF EXISTS `user`;
CREATE TABLE IF NOT EXISTS `user` (
  `user_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `name` varchar(32) NOT NULL COMMENT 'unique user name',
  `email` varchar(64) NOT NULL COMMENT 'user e-mail',
  `auth_type` enum('Local') NOT NULL DEFAULT 'Local' COMMENT 'type of authorization',
  `auth_data` varchar(64) NOT NULL DEFAULT '' COMMENT 'authorization data depending on a store',
  `salt` varchar(64) NOT NULL DEFAULT '' COMMENT 'hash salt (for local auth only)',
  `promocode` varchar(8) NOT NULL DEFAULT '' COMMENT 'promo code suffix to invite new users',
  `character` enum('Rabbit','Hedgehog','Squirrel','Cat') NOT NULL DEFAULT 'Rabbit' COMMENT 'user appearance',
  `gems` int(10) unsigned NOT NULL DEFAULT '0' COMMENT 'gems count',
  `trust_points` int(10) unsigned NOT NULL DEFAULT '20' COMMENT 'trust points count',
  `last_enemy` bigint(20) unsigned DEFAULT NULL COMMENT 'last enemy user_id',
  `agent_info` varchar(64) NOT NULL DEFAULT '' COMMENT 'agent information (version, platform, language, etc.)',
  `last_login` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'last login time',
  PRIMARY KEY (`user_id`),
  UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='user table';

-- Data exporting was unselected.


-- Dumping structure for table rush.user_ability
DROP TABLE IF EXISTS `user_ability`;
CREATE TABLE IF NOT EXISTS `user_ability` (
  `user_ability_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `user_id` bigint(20) unsigned NOT NULL COMMENT 'reference to a user (stackoverflow.com/questions/304317)',
  `name` enum('Snorkel','ClimbingShoes','SouthWester','VoodooMask','SapperShoes','Sunglasses','7','8','9','10','11','12','13','14','15','16','17','SpPack2','19','20','21','22','23','24','25','26','27','28','29','30','31','32','Miner','Builder','Shaman','Grenadier','TeleportMan') NOT NULL DEFAULT 'Snorkel' COMMENT 'ability name (DO NOT use ability_id, because it causes bugs when one buys in addition the same ability but with the other duration)',
  `expire` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'time when the ability get expired',
  PRIMARY KEY (`user_ability_id`),
  UNIQUE KEY `user_ability` (`user_id`,`name`),
  KEY `expire` (`expire`),
  CONSTRAINT `user_id` FOREIGN KEY (`user_id`) REFERENCES `user` (`user_id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='user ability timestamps';

-- Data exporting was unselected.


-- Dumping structure for trigger rush.before_friend_insert
DROP TRIGGER IF EXISTS `before_friend_insert`;
SET @OLDTMP_SQL_MODE=@@SQL_MODE, SQL_MODE='STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION';
DELIMITER //
CREATE TRIGGER `before_friend_insert` BEFORE INSERT ON `friend` FOR EACH ROW BEGIN
CALL sp_friend(NEW.user_id, NEW.friend_user_id);
END//
DELIMITER ;
SET SQL_MODE=@OLDTMP_SQL_MODE;


-- Dumping structure for trigger rush.before_friend_update
DROP TRIGGER IF EXISTS `before_friend_update`;
SET @OLDTMP_SQL_MODE=@@SQL_MODE, SQL_MODE='STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION';
DELIMITER //
CREATE TRIGGER `before_friend_update` BEFORE UPDATE ON `friend` FOR EACH ROW BEGIN
CALL sp_friend(NEW.user_id, NEW.friend_user_id);
END//
DELIMITER ;
SET SQL_MODE=@OLDTMP_SQL_MODE;


-- Dumping structure for trigger rush.before_user_insert
DROP TRIGGER IF EXISTS `before_user_insert`;
SET @OLDTMP_SQL_MODE=@@SQL_MODE, SQL_MODE='STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION';
DELIMITER //
CREATE TRIGGER `before_user_insert` BEFORE INSERT ON `user` FOR EACH ROW BEGIN
CALL sp_user(NEW.name, NEW.email);
END//
DELIMITER ;
SET SQL_MODE=@OLDTMP_SQL_MODE;


-- Dumping structure for trigger rush.before_user_update
DROP TRIGGER IF EXISTS `before_user_update`;
SET @OLDTMP_SQL_MODE=@@SQL_MODE, SQL_MODE='STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION';
DELIMITER //
CREATE TRIGGER `before_user_update` BEFORE UPDATE ON `user` FOR EACH ROW BEGIN
CALL sp_user(NEW.name, NEW.email);
END//
DELIMITER ;
SET SQL_MODE=@OLDTMP_SQL_MODE;
/*!40101 SET SQL_MODE=IFNULL(@OLD_SQL_MODE, '') */;
/*!40014 SET FOREIGN_KEY_CHECKS=IF(@OLD_FOREIGN_KEY_CHECKS IS NULL, 1, @OLD_FOREIGN_KEY_CHECKS) */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
